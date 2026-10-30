package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
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

func (m *Mint) VerifyAuthBlindToken(authProof cashu.AuthProof) error {
	ctx := context.Background()

	y, err := authProof.Y()
	if err != nil {
		return fmt.Errorf("authProof.Y(). %w", err)
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

	proofsList, err := m.MintDB.GetProofsFromSecretCurve(tx, []cashu.WrappedPublicKey{y})
	if err != nil {
		return fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, []string{y} ). %w", err)
	}
	if len(proofsList) > 0 {
		return errors.New("authProof already used")
	}

	proof := authProof.Proof(y, cashu.PROOF_PENDING)
	proofArray := cashu.Proofs{proof}
	keysets, err := m.Signer.GetAuthKeys()
	if err != nil {
		return fmt.Errorf("m.Signer.GetAuthKeys(). %w", err)
	}
	unit, err := checkProofsAreSameUnit(proofArray, keysets.Keysets)
	if err != nil {
		return fmt.Errorf("checkProofsAreSameUnit(proofArray, keysets.Keysets). %w", err)
	}
	if unit != cashu.AUTH {
		return cashu.ErrUnitNotSupported
	}

	err = m.MintDB.SaveProof(tx, proofArray)
	if err != nil {
		return fmt.Errorf("m.MintDB.SaveProof(tx, proofArray). %w", err)
	}

	err = m.Signer.VerifyProofs(proofArray)
	if err != nil {
		return fmt.Errorf("m.Signer.VerifyProofs(proofArray, nil). %w", err)
	}

	proofArray.SetProofsState(cashu.PROOF_SPENT)

	err = m.MintDB.SetProofsState(tx, proofArray, cashu.PROOF_SPENT)
	if err != nil {
		return fmt.Errorf("m.MintDB.SetProofsState(tx, proofArray, cashu.PROOF_SPENT). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	return nil
}
