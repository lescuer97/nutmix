//nolint:exhaustruct // Test fixtures intentionally initialize only fields used by these routes.
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/internal/utils"
)

type authFailureDB struct {
	database.MintDB
	failure string
}

func (db authFailureDB) GetTx(context.Context) (pgx.Tx, error) {
	if db.failure == "get transaction" {
		return nil, errors.New("private database detail")
	}
	return nil, nil
}

func (authFailureDB) GetRestoreSigsFromBlindedMessages(pgx.Tx, []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error) {
	return nil, nil
}

func (db authFailureDB) SaveRestoreSigs(pgx.Tx, []cashu.RecoverSigDB) error {
	if db.failure == "save signatures" {
		return errors.New("private database detail")
	}
	return nil
}

func (db authFailureDB) Commit(context.Context, pgx.Tx) error {
	if db.failure == "commit" {
		return errors.New("private database detail")
	}
	return nil
}

func (authFailureDB) Rollback(context.Context, pgx.Tx) error { return nil }

type authFailureSigner struct{ signer.Signer }

func (authFailureSigner) GetAuthKeys() (signer.GetKeysetsResponse, error) {
	return signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: "auth-keyset", Unit: "auth"}}}, nil
}

func (authFailureSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	return []cashu.BlindSignature{{Id: messages[0].Id, Amount: messages[0].Amount}}, nil, nil
}

func TestAuthMintDatabaseGetTxFailureReturnsServerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	body, err := json.Marshal(cashu.PostMintBolt11Request{Outputs: cashu.BlindedMessages{{Id: "auth-keyset", Amount: 1, B_: cashu.WrappedPublicKey{PublicKey: pub}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"get transaction", "save signatures", "commit"} {
		t.Run(failure, func(t *testing.T) {
			mint := &m.Mint{MintDB: authFailureDB{failure: failure}, Signer: authFailureSigner{}, Config: utils.Config{MINT_REQUIRE_AUTH: true, MINT_AUTH_MAX_BLIND_TOKENS: 1}}
			router := gin.New()
			v1AuthRoutes(router, mint)
			req := httptest.NewRequest("POST", "/v1/auth/blind/mint", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 500 || response.Body.Len() == 0 || strings.Contains(response.Body.String(), "private database detail") {
				t.Fatalf("expected generic HTTP 500 response; got %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestAuthMintMalformedRequestRemainsClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mint := &m.Mint{MintDB: authFailureDB{failure: "get transaction"}, Config: utils.Config{MINT_REQUIRE_AUTH: true, MINT_AUTH_MAX_BLIND_TOKENS: 1}}
	router := gin.New()
	v1AuthRoutes(router, mint)
	req := httptest.NewRequest("POST", "/v1/auth/blind/mint", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 400 {
		t.Fatalf("expected malformed request HTTP 400; got %d %q", response.Code, response.Body.String())
	}
}
