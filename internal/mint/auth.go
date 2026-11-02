package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func (m *Mint) verifyClams(clams cashu.AuthClams) error {
	ctx := context.Background()

	if clams.Sub == "" {
		return cashu.ErrInvalidAuthToken
	}
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("rollback error", slog.Any("error", rollbackErr))
		}
	}()

	now := time.Now()
	authUser, err := m.MintDB.GetAuthUser(tx, clams.Sub)
	if err != nil {
		// if the user doesn't exist we create it
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("m.MintDB.GetAuthUser(tx, clams.Sub). %w", err)
		}
		authUser.Sub = clams.Sub
		authUser.LastLoggedIn = uint64(now.Unix())
		err = m.MintDB.MakeAuthUser(tx, authUser)
		if err != nil {
			return fmt.Errorf("m.MintDB.MakeAuthUser(tx, authUser). %w", err)
		}
	}

	authUser.LastLoggedIn = uint64(now.Unix())
	err = m.MintDB.UpdateLastLoggedIn(tx, authUser.Sub, authUser.LastLoggedIn)
	if err != nil {
		return fmt.Errorf("m.MintDB.UpdateLastLoggedIn(tx,authUser.Sub, authUser.LastLoggedIn). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	return nil
}

func (m *Mint) VerifyAuthClearToken(token string) error {
	verifier := m.OICDClient.Verifier(&oidc.Config{ClientID: m.Config.MINT_AUTH_OICD_CLIENT_ID, Now: time.Now, SkipClientIDCheck: true}) //nolint:exhaustruct

	ctx := context.Background()
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return fmt.Errorf("verifier.Verify(ctx,token ). %w", err)
	}
	now := time.Now()
	if now.Unix() >= idToken.Expiry.Unix() {
		return cashu.ErrClearTokenExpired
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	err = idToken.Claims(&claims)
	if err != nil {
		return fmt.Errorf("idToken.Claims(&claims). %w", err)
	}
	if len(idToken.Audience) > 0 {
		audienceMatches := false
		for _, audience := range idToken.Audience {
			if audience == m.MintPubkey || audience == m.Config.MINT_AUTH_OICD_CLIENT_ID {
				audienceMatches = true
				break
			}
		}
		if !audienceMatches {
			return cashu.ErrInvalidAuthToken
		}
	}
	if claims.Sub == "" {
		return cashu.ErrInvalidAuthToken
	}
	err = m.verifyClams(cashu.AuthClams{Sub: claims.Sub}) //nolint:exhaustruct
	if err != nil {
		return fmt.Errorf("m.verifyClams(clams). %w", err)
	}

	return nil
}

func (m *Mint) ReserveAuthBlindToken(authProof cashu.AuthProof) (cashu.Proof, error) {
	ctx := context.Background()
	if m.MintDB == nil || m.Signer == nil || authProof.C.PublicKey == nil || authProof.Id == "" || !utf8.ValidString(authProof.Secret) {
		return cashu.Proof{}, cashu.ErrInvalidAuthToken
	}
	authProof.Amount = 1

	y, err := authProof.Y()
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("authProof.Y(). %w", err)
	}

	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("rollback error", slog.Any("error", rollbackErr))
		}
	}()

	proofsList, err := m.MintDB.GetProofsFromSecretCurve(tx, []cashu.WrappedPublicKey{y})
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, []string{y} ). %w", err)
	}
	if len(proofsList) > 0 {
		return cashu.Proof{}, errors.New("authProof already used")
	}

	proof := authProof.Proof(y, cashu.PROOF_PENDING)
	proofArray := cashu.Proofs{proof}
	keysets, err := m.Signer.GetAuthKeys()
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("m.Signer.GetAuthKeys(). %w", err)
	}
	unit, err := checkProofsAreSameUnit(proofArray, keysets.Keysets)
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("checkProofsAreSameUnit(proofArray, keysets.Keysets). %w", err)
	}
	if unit != cashu.AUTH {
		return cashu.Proof{}, cashu.ErrUnitNotSupported
	}
	reservationID, err := uuid.NewRandom()
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("uuid.NewRandom(). %w", err)
	}
	reservation := reservationID.String()
	proof.Quote = &reservation
	proofArray[0].Quote = &reservation

	err = m.MintDB.SaveProof(tx, proofArray)
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("m.MintDB.SaveProof(tx, proofArray). %w", err)
	}

	err = m.Signer.VerifyProofs(proofArray)
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("m.Signer.VerifyProofs(proofArray, nil). %w", err)
	}
	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return cashu.Proof{}, fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}
	return proof, nil
}

// FinishAuthBlindToken spends an owned reservation on success and releases it on failure.
func (m *Mint) FinishAuthBlindToken(proof cashu.Proof, success bool) error {
	if proof.Quote == nil || *proof.Quote == "" {
		return cashu.ErrInvalidAuthToken
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("finalize auth blind token reservation: %w", err)
		}
		tx, err := m.MintDB.GetTx(ctx)
		if err != nil {
			if attempt == 2 || ctx.Err() != nil {
				return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
			}
			if err := waitForAuthFinalizationRetry(ctx); err != nil {
				return fmt.Errorf("finalize auth blind token reservation: %w", err)
			}
			continue
		}
		stored, err := m.MintDB.GetProofsFromSecretCurve(tx, []cashu.WrappedPublicKey{proof.Y})
		if err == nil {
			owned := len(stored) == 1 && stored[0].Quote != nil && *stored[0].Quote == *proof.Quote
			if !owned {
				_ = m.MintDB.Rollback(ctx, tx)
				if success {
					return errors.New("auth proof reservation ownership changed")
				}
				return nil
			}
			if success && stored[0].State == cashu.PROOF_SPENT {
				_ = m.MintDB.Rollback(ctx, tx)
				return nil
			}
			if stored[0].State != cashu.PROOF_PENDING {
				_ = m.MintDB.Rollback(ctx, tx)
				if success {
					return errors.New("auth proof reservation is not pending")
				}
				return nil
			}
			if success {
				stored.SetProofsState(cashu.PROOF_SPENT)
				err = m.MintDB.SetProofsState(tx, stored, cashu.PROOF_SPENT)
			} else {
				err = m.MintDB.DeleteProofs(tx, stored)
			}
		}
		if err == nil {
			err = m.MintDB.Commit(ctx, tx)
		} else {
			_ = m.MintDB.Rollback(ctx, tx)
		}
		if err == nil {
			return nil
		}
		_ = m.MintDB.Rollback(ctx, tx)
		if attempt == 2 || ctx.Err() != nil {
			return fmt.Errorf("finalize auth blind token reservation: %w", err)
		}
		if err := waitForAuthFinalizationRetry(ctx); err != nil {
			return fmt.Errorf("finalize auth blind token reservation: %w", err)
		}
	}
	return errors.New("finalize auth blind token reservation failed")
}

func waitForAuthFinalizationRetry(ctx context.Context) error {
	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *Mint) VerifyAuthBlindToken(authProof cashu.AuthProof) error {
	proof, err := m.ReserveAuthBlindToken(authProof)
	if err != nil {
		return err
	}
	return m.FinishAuthBlindToken(proof, true)
}
