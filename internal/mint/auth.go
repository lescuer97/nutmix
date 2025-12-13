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

	if clams.ClientId != m.Config.MINT_AUTH_OICD_CLIENT_ID {
		return cashu.ErrInvalidAuthToken
	}
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		if err := m.MintDB.Rollback(ctx, tx); err != nil {
			slog.Warn("rollback error", slog.Any("error", err))
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
	verifier := m.OICDClient.Verifier(&oidc.Config{ClientID: m.Config.MINT_AUTH_OICD_CLIENT_ID, Now: time.Now, SkipClientIDCheck: false})

	ctx := context.Background()
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return fmt.Errorf("verifier.Verify(ctx,token ). %w", err)
	}
	now := time.Now()
	if now.Unix() >= idToken.Expiry.Unix() {
		return cashu.ErrClearTokenExpired
	}
	clams := cashu.AuthClams{}
	err = idToken.Claims(&clams)
	if err != nil {
		return fmt.Errorf("idToken.Claims(&clams). %w", err)
	}
	err = m.verifyClams(clams)
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
		if err := m.MintDB.Rollback(ctx, tx); err != nil {
			slog.Warn("rollback error", slog.Any("error", err))
		}
	}()

	proofsList, err := m.MintDB.GetProofsFromSecretCurve(tx, []cashu.WrappedPublicKey{y})
	if err != nil {
		return fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, []string{y} ). %w", err)
	}
	if len(proofsList) > 0 {
		return fmt.Errorf("authProof already used. %w", err)
	}

	proof := authProof.Proof(y, cashu.PROOF_PENDING)
	proofArray := cashu.Proofs{proof}
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
		return fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, []string{y} ). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	return nil
}
