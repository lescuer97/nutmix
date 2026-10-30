package mint

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
	mockdb "github.com/lescuer97/nutmix/internal/database/mock_db"
	"github.com/lescuer97/nutmix/internal/utils"
)

type catComplianceDB struct {
	*mockdb.MockDB
}

func (db *catComplianceDB) GetAuthUser(_ pgx.Tx, sub string) (database.AuthUser, error) {
	return database.AuthUser{Sub: sub}, nil //nolint:exhaustruct
}

func (*catComplianceDB) MakeAuthUser(_ pgx.Tx, _ database.AuthUser) error { return nil }

func (*catComplianceDB) UpdateLastLoggedIn(_ pgx.Tx, _ string, _ uint64) error { return nil }

func TestCATClaimsCompliance(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": server.URL, "jwks_uri": server.URL + "/jwks",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
				"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := oidc.NewProvider(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:exhaustruct
	m := &Mint{
		OICDClient: provider,
		MintDB:     &catComplianceDB{MockDB: &mockdb.MockDB{}}, //nolint:exhaustruct
		MintPubkey: "mintkey",
		Config:     utils.Config{MINT_AUTH_OICD_CLIENT_ID: "client"}, //nolint:exhaustruct
	}

	//nolint:govet,prealloc
	tests := []struct {
		name    string
		claims  jwt.MapClaims
		key     *rsa.PrivateKey
		wantErr bool
	}{
		{name: "absent audience accepted", claims: jwt.MapClaims{"sub": "user"}},                                                                          //nolint:exhaustruct
		{name: "client audience accepted", claims: jwt.MapClaims{"sub": "user", "aud": "client"}},                                                         //nolint:exhaustruct
		{name: "mint audience accepted", claims: jwt.MapClaims{"sub": "user", "aud": "mintkey"}},                                                          //nolint:exhaustruct
		{name: "multiple audiences including mint accepted", claims: jwt.MapClaims{"sub": "user", "aud": []string{"other", "mintkey"}}},                   //nolint:exhaustruct
		{name: "wrong audience rejected", claims: jwt.MapClaims{"sub": "user", "aud": "other"}, wantErr: true},                                            //nolint:exhaustruct
		{name: "missing subject rejected", claims: jwt.MapClaims{"aud": "mintkey"}, wantErr: true},                                                        //nolint:exhaustruct
		{name: "empty subject rejected", claims: jwt.MapClaims{"sub": "", "aud": "mintkey"}, wantErr: true},                                               //nolint:exhaustruct
		{name: "expired token rejected", claims: jwt.MapClaims{"sub": "user", "exp": time.Now().Add(-time.Hour).Unix(), "aud": "mintkey"}, wantErr: true}, //nolint:exhaustruct
		{name: "wrong issuer rejected", claims: jwt.MapClaims{"iss": "https://wrong.example", "sub": "user", "aud": "mintkey"}, wantErr: true},            //nolint:exhaustruct
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:govet
	tests = append(tests, struct {
		name    string
		claims  jwt.MapClaims
		key     *rsa.PrivateKey
		wantErr bool
	}{name: "bad signature rejected", claims: jwt.MapClaims{"sub": "user", "aud": "mintkey"}, key: wrongKey, wantErr: true}) //nolint:exhaustruct

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": server.URL, "exp": time.Now().Add(time.Hour).Unix()}
			for claim, value := range tc.claims {
				claims[claim] = value
			}
			key := tc.key
			if key == nil {
				key = privateKey
			}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = "test"
			encoded, err := token.SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			err = m.VerifyAuthClearToken(encoded)
			if (err != nil) != tc.wantErr {
				t.Fatalf("VerifyAuthClearToken() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
