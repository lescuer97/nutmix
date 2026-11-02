//nolint:exhaustruct // Test fixtures intentionally specify only relevant fields.
package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/signer"
)

type batLifecycleSigner struct {
	signer.Signer
	keysetID string
}

func (s batLifecycleSigner) GetAuthKeys() (signer.GetKeysetsResponse, error) {
	return signer.GetKeysetsResponse{Keysets: []cashu.BasicKeysetResponse{{Id: s.keysetID, Unit: "auth"}}}, nil
}
func (batLifecycleSigner) VerifyProofs([]cashu.Proof) error { return nil }

type batLifecycleDB struct { //nolint:govet // Test fixture keeps synchronized proof state together.
	database.MintDB
	mu     sync.Mutex
	proofs []cashu.Proof
}

func (*batLifecycleDB) GetTx(context.Context) (pgx.Tx, error)  { return nil, nil }
func (*batLifecycleDB) Commit(context.Context, pgx.Tx) error   { return nil }
func (*batLifecycleDB) Rollback(context.Context, pgx.Tx) error { return nil }
func (db *batLifecycleDB) GetProofsFromSecretCurve(_ pgx.Tx, ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	wanted := map[string]bool{}
	for _, y := range ys {
		wanted[y.ToHex()] = true
	}
	var out cashu.Proofs
	for _, p := range db.proofs {
		if wanted[p.Y.ToHex()] {
			out = append(out, p)
		}
	}
	return out, nil
}
func (db *batLifecycleDB) SaveProof(_ pgx.Tx, ps []cashu.Proof) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, p := range ps {
		for _, old := range db.proofs {
			if old.Y.ToHex() == p.Y.ToHex() {
				return fmt.Errorf("proof already exists")
			}
		}
	}
	db.proofs = append(db.proofs, ps...)
	return nil
}
func (db *batLifecycleDB) SetProofsState(_ pgx.Tx, ps cashu.Proofs, state cashu.ProofState) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for i := range db.proofs {
		for _, p := range ps {
			if db.proofs[i].Y.ToHex() == p.Y.ToHex() {
				db.proofs[i].State = state
			}
		}
	}
	return nil
}
func (db *batLifecycleDB) DeleteProofs(_ pgx.Tx, ps cashu.Proofs) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, p := range ps {
		for i := 0; i < len(db.proofs); i++ {
			if db.proofs[i].Y.ToHex() == p.Y.ToHex() {
				db.proofs = append(db.proofs[:i], db.proofs[i+1:]...)
				break
			}
		}
	}
	return nil
}
func (db *batLifecycleDB) snapshot() cashu.Proofs {
	db.mu.Lock()
	defer db.mu.Unlock()
	return append(cashu.Proofs(nil), db.proofs...)
}

func newBATLifecycle(t *testing.T, secret string) (*batLifecycleDB, string, *mint.Mint) {
	t.Helper()
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	db := &batLifecycleDB{}
	const id = "auth-keyset"
	proof := cashu.AuthProof{C: cashu.WrappedPublicKey{PublicKey: pub}, Id: id, Secret: secret}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	token := "authA" + base64.URLEncoding.EncodeToString(encoded)
	m := &mint.Mint{MintDB: db, Signer: batLifecycleSigner{keysetID: id}}
	m.Config.MINT_REQUIRE_AUTH = true
	m.Config.MINT_AUTH_BLIND_AUTH_URLS = []string{"/protected"}
	return db, token, m
}
func requestBAT(r http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Blind-auth", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBATLifecycleSuccessSpendsProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, token, m := newBATLifecycle(t, "success")
	r := gin.New()
	r.Use(BlindAuthMiddleware(m))
	r.GET("/protected", func(c *gin.Context) { c.String(200, "ok") })
	w := requestBAT(r, token)
	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("response=%d %q", w.Code, w.Body.String())
	}
	ps := db.snapshot()
	if len(ps) != 1 || ps[0].State != cashu.PROOF_SPENT {
		t.Fatalf("proofs=%#v", ps)
	}
}

func TestBATLifecycleFailuresReleaseReservation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		handler gin.HandlerFunc
		name    string
	}{{func(c *gin.Context) { c.Status(400) }, "status"}, {func(c *gin.Context) { _ = c.Error(fmt.Errorf("handler error")); c.Status(200) }, "gin error"}} {
		t.Run(tc.name, func(t *testing.T) {
			db, token, m := newBATLifecycle(t, tc.name)
			r := gin.New()
			r.Use(BlindAuthMiddleware(m))
			r.GET("/protected", tc.handler)
			w := requestBAT(r, token)
			if w.Code < 200 {
				t.Fatalf("status=%d", w.Code)
			}
			if got := db.snapshot(); len(got) != 0 {
				t.Fatalf("reservation retained: %#v", got)
			}
			retry := gin.New()
			retry.Use(BlindAuthMiddleware(m))
			retry.GET("/protected", func(c *gin.Context) { c.Status(200) })
			if w := requestBAT(retry, token); w.Code != 200 {
				t.Fatalf("retry status=%d", w.Code)
			}
		})
	}
}

func TestBATLifecycleRejectsMalformedOrMissingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, token, m := newBATLifecycle(t, "bad")
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {"auth", 400}} {
		r := gin.New()
		r.Use(BlindAuthMiddleware(m))
		r.GET("/protected", func(c *gin.Context) { c.Status(200) })
		if w := requestBAT(r, tc.token); w.Code != tc.status {
			t.Fatalf("token %q status=%d want %d", tc.token, w.Code, tc.status)
		}
	}
	_ = token
}

func TestBATLifecycleRejectsNullPointHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, m := newBATLifecycle(t, "null-point")
	encoded, err := json.Marshal(cashu.AuthProof{Id: "auth-keyset", Secret: "null-point"})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(BlindAuthMiddleware(m))
	r.GET("/protected", func(c *gin.Context) { c.Status(200) })
	if w := requestBAT(r, "authA"+base64.URLEncoding.EncodeToString(encoded)); w.Code != 400 {
		t.Fatalf("response status=%d, want 400", w.Code)
	}
}

func TestBATLifecycleConcurrentReservationRejectsSecondRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, token, m := newBATLifecycle(t, "concurrent")
	entered, release := make(chan struct{}), make(chan struct{})
	r := gin.New()
	r.Use(BlindAuthMiddleware(m))
	r.GET("/protected", func(c *gin.Context) { close(entered); <-release; c.Status(400) })
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Blind-auth", token)
		r.ServeHTTP(first, req)
	}()
	<-entered
	if w := requestBAT(r, token); w.Code != 400 {
		t.Fatalf("second status=%d want 400", w.Code)
	}
	close(release)
	<-done
	if got := db.snapshot(); len(got) != 0 {
		t.Fatalf("reservation retained: %#v", got)
	}
}
