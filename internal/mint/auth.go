package mint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
)

func (m *Mint) verifyClams(ctx context.Context, clams cashu.AuthClams) error {
	if clams.ClientId != m.Config.MINT_AUTH_OICD_CLIENT_ID {
		return cashu.ErrInvalidAuthToken
	}
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer m.rollbackIndependent(ctx, tx)

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

func (m *Mint) VerifyAuthClearToken(ctx context.Context, token string) error {
	verifier := m.OICDClient.Verifier(&oidc.Config{ClientID: m.Config.MINT_AUTH_OICD_CLIENT_ID, Now: time.Now, SkipClientIDCheck: false}) //nolint:exhaustruct_v5

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return fmt.Errorf("verifier.Verify(ctx,token ). %w", err)
	}
	now := time.Now()
	if now.Unix() >= idToken.Expiry.Unix() {
		return cashu.ErrClearTokenExpired
	}
	clams := cashu.AuthClams{
		Aud:      nil,
		Sub:      "",
		ClientId: "",
	}
	err = idToken.Claims(&clams)
	if err != nil {
		return fmt.Errorf("idToken.Claims(&clams). %w", err)
	}
	err = m.verifyClams(ctx, clams)
	if err != nil {
		return fmt.Errorf("m.verifyClams(clams). %w", err)
	}

	return nil
}

func (m *Mint) VerifyAuthBlindToken(ctx context.Context, authProof cashu.AuthProof) error {
	y, err := authProof.Y()
	if err != nil {
		return fmt.Errorf("authProof.Y(). %w", err)
	}

	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer m.rollbackIndependent(ctx, tx)

	proofsList, err := m.MintDB.GetProofsFromSecretCurve(ctx, tx, []cashu.WrappedPublicKey{y})
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

	err = m.MintDB.SaveProof(ctx, tx, proofArray)
	if err != nil {
		return fmt.Errorf("m.MintDB.SaveProof(tx, proofArray). %w", err)
	}

	err = m.Signer.VerifyProofs(proofArray)
	if err != nil {
		return fmt.Errorf("m.Signer.VerifyProofs(proofArray, nil). %w", err)
	}

	proofArray.SetProofsState(cashu.PROOF_SPENT)

	err = m.MintDB.SetProofsState(ctx, tx, proofArray, cashu.PROOF_SPENT)
	if err != nil {
		return fmt.Errorf("m.MintDB.SetProofsState(tx, proofArray, cashu.PROOF_SPENT). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	return nil
}
