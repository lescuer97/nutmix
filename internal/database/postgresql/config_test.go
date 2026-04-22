package postgresql

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func mustWrappedPubkey(t *testing.T, hexPubkey string) cashu.WrappedPublicKey {
	t.Helper()

	pubkeyBytes, err := hex.DecodeString(hexPubkey)
	if err != nil {
		t.Fatalf("hex.DecodeString(hexPubkey): %v", err)
	}

	pubkey, err := secp256k1.ParsePubKey(pubkeyBytes)
	if err != nil {
		t.Fatalf("secp256k1.ParsePubKey(pubkeyBytes): %v", err)
	}

	return cashu.WrappedPublicKey{PublicKey: pubkey}
}

func commitConfigTx(t *testing.T, db Postgresql, fn func(tx pgx.Tx) error) {
	t.Helper()

	ctx := context.Background()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}

	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	err = fn(tx)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Commit(ctx, tx); err != nil {
		t.Fatalf("db.Commit(ctx, tx): %v", err)
	}
}

func TestUpdateNostrNotificationConfig_PersistsNpubsAndFlags(t *testing.T) {
	db, _ := setupTestDB(t)

	var config utils.Config
	config.Default()

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.SetConfig(tx, config)
	})

	npub1 := mustWrappedPubkey(t, "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac")
	npub2 := mustWrappedPubkey(t, "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2")

	nostrConfig := utils.NostrNotificationConfig{}
	if err := nostrConfig.SetNostrNotificationConfig(true, nil, []cashu.WrappedPublicKey{npub1, npub2}); err != nil {
		t.Fatalf("nostrConfig.SetNostrNotificationConfig(...): %v", err)
	}
	nostrConfig.NOSTR_NOTIFICATION_NIP04_DM = true

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.UpdateNostrNotificationConfig(tx, nostrConfig)
	})

	ctx := context.Background()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	updatedConfig, err := db.GetConfig(tx)
	if err != nil {
		t.Fatalf("db.GetConfig(tx): %v", err)
	}

	updatedNostrConfig, err := db.GetNostrNotificationConfig(tx)
	if err != nil {
		t.Fatalf("db.GetNostrNotificationConfig(tx): %v", err)
	}

	if updatedConfig.NOSTR != config.NOSTR {
		t.Fatalf("unexpected general config mutation: got %q want %q", updatedConfig.NOSTR, config.NOSTR)
	}

	if updatedNostrConfig == nil {
		t.Fatal("expected nostr notification config row")
	}

	if !updatedNostrConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected nostr notifications to be enabled")
	}

	if !updatedNostrConfig.NOSTR_NOTIFICATION_NIP04_DM {
		t.Fatal("expected nostr notification NIP-04 DM flag to be enabled")
	}

	if len(updatedNostrConfig.NOSTR_NOTIFICATION_NSEC) != 0 {
		t.Fatal("expected nostr notification nsec to stay out of database reads")
	}

	if len(updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS) != 2 {
		t.Fatalf("expected 2 stored npubs, got %d", len(updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	if updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex() != npub1.ToHex() {
		t.Fatalf("first stored npub mismatch: got %s want %s", updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex(), npub1.ToHex())
	}

	if updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS[1].ToHex() != npub2.ToHex() {
		t.Fatalf("second stored npub mismatch: got %s want %s", updatedNostrConfig.NOSTR_NOTIFICATION_NPUBS[1].ToHex(), npub2.ToHex())
	}
}

func TestGetNostrNotificationConfig_ReturnsNilWhenRowMissing(t *testing.T) {
	db, _ := setupTestDB(t)

	ctx := context.Background()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	nostrConfig, err := db.GetNostrNotificationConfig(tx)
	if err != nil {
		t.Fatalf("db.GetNostrNotificationConfig(tx): %v", err)
	}

	if nostrConfig != nil {
		t.Fatal("expected nil nostr notification config when table is empty")
	}
}

func TestUpdateConfig_DoesNotPersistNostrNotificationFields(t *testing.T) {
	db, _ := setupTestDB(t)

	var config utils.Config
	config.Default()

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.SetConfig(tx, config)
	})

	config.NAME = "updated-name"

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.UpdateConfig(tx, config)
	})

	ctx := context.Background()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	updatedConfig, err := db.GetConfig(tx)
	if err != nil {
		t.Fatalf("db.GetConfig(tx): %v", err)
	}

	nostrConfig, err := db.GetNostrNotificationConfig(tx)
	if err != nil {
		t.Fatalf("db.GetNostrNotificationConfig(tx): %v", err)
	}

	if updatedConfig.NAME != "updated-name" {
		t.Fatalf("expected general config update to persist, got %q", updatedConfig.NAME)
	}

	if nostrConfig != nil {
		t.Fatal("expected nostr notification table to remain empty after general config update")
	}
}

func TestUpdateNostrNotificationConfig_PreservesDisabledRow(t *testing.T) {
	db, _ := setupTestDB(t)

	var config utils.Config
	config.Default()

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.SetConfig(tx, config)
	})

	npub := mustWrappedPubkey(t, "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac")
	nostrConfig := utils.NostrNotificationConfig{}
	if err := nostrConfig.SetNostrNotificationConfig(false, nil, []cashu.WrappedPublicKey{npub}); err != nil {
		t.Fatalf("nostrConfig.SetNostrNotificationConfig(...): %v", err)
	}

	commitConfigTx(t, db, func(tx pgx.Tx) error {
		return db.UpdateNostrNotificationConfig(tx, nostrConfig)
	})

	ctx := context.Background()
	tx, err := db.GetTx(ctx)
	if err != nil {
		t.Fatalf("db.GetTx(ctx): %v", err)
	}
	defer func() {
		_ = db.Rollback(ctx, tx)
	}()

	loadedConfig, err := db.GetNostrNotificationConfig(tx)
	if err != nil {
		t.Fatalf("db.GetNostrNotificationConfig(tx): %v", err)
	}

	if loadedConfig == nil {
		t.Fatal("expected disabled nostr notification row to remain present")
	}

	if loadedConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected nostr notifications to remain disabled")
	}

	if len(loadedConfig.NOSTR_NOTIFICATION_NPUBS) != 1 {
		t.Fatalf("expected 1 stored npub, got %d", len(loadedConfig.NOSTR_NOTIFICATION_NPUBS))
	}
}
