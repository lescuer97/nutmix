package mint

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

type authPatternConfigDB struct {
	database.MintDB
	config       utils.Config
	updated      utils.Config
	updateCalled bool
	committed    bool
}

func (*authPatternConfigDB) GetTx(context.Context) (pgx.Tx, error)     { return nil, nil }
func (*authPatternConfigDB) Rollback(context.Context, pgx.Tx) error    { return nil }
func (db *authPatternConfigDB) Commit(context.Context, pgx.Tx) error   { db.committed = true; return nil }
func (db *authPatternConfigDB) GetConfig(pgx.Tx) (utils.Config, error) { return db.config, nil }
func (*authPatternConfigDB) GetNostrNotificationConfig(pgx.Tx) (*utils.NostrNotificationConfig, error) {
	return nil, nil
}
func (db *authPatternConfigDB) UpdateConfig(_ pgx.Tx, config utils.Config) error {
	db.updated, db.updateCalled = config, true
	return nil
}

func TestConfigAuthPatternsRemainCanonicalWithoutRewrite(t *testing.T) {
	config := utils.Config{ //nolint:exhaustruct // only auth endpoint configuration is relevant
		MINT_REQUIRE_AUTH:         true,
		MINT_AUTH_BLIND_AUTH_URLS: []string{"/v1/mint/*", "/v1/melt/*"},
		MINT_AUTH_CLEAR_AUTH_URLS: []string{"/v1/mint/*", "/v1/melt/*"},
	}
	db := &authPatternConfigDB{config: config} //nolint:exhaustruct // other fields are observed after setup
	got, _, err := SetUpConfigDB(context.Background(), db)
	if err != nil {
		t.Fatalf("SetUpConfigDB: %v", err)
	}
	if len(got.MINT_AUTH_BLIND_AUTH_URLS) != 2 || got.MINT_AUTH_BLIND_AUTH_URLS[0] != "/v1/mint/*" {
		t.Errorf("returned patterns = %v", got.MINT_AUTH_BLIND_AUTH_URLS)
	}
	if db.updateCalled {
		t.Fatal("startup must not rewrite already-canonical config")
	}
	if !db.committed {
		t.Fatal("expected config transaction to commit")
	}
}

func TestConfigAuthPatternsRejectUnsupportedRegex(t *testing.T) {
	db := &authPatternConfigDB{config: utils.Config{ //nolint:exhaustruct // only auth endpoint configuration is relevant
		MINT_REQUIRE_AUTH: true, MINT_AUTH_BLIND_AUTH_URLS: []string{"^/v1/(mint|melt)/.*"},
	}} //nolint:exhaustruct // only config is relevant
	if _, _, err := SetUpConfigDB(context.Background(), db); err == nil {
		t.Fatal("expected unsupported regex to fail startup config")
	}
	if db.updateCalled || db.committed {
		t.Fatal("unsupported regex must not be persisted or committed")
	}
}
