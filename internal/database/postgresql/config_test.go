package postgresql

import (
	"encoding/hex"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
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

func TestUpdateNostrNotificationConfig_PersistsNpubsAndFlags(t *testing.T) {
	db, _ := setupTestDB(t)

	var config utils.Config
	config.Default()

	if err := db.SetConfig(config); err != nil {
		t.Fatalf("db.SetConfig(config): %v", err)
	}

	npub1 := mustWrappedPubkey(t, "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac")
	npub2 := mustWrappedPubkey(t, "02a9acc1e48c25eeeb9289b5031cc57da9fe72f3fe2861d264bdc074209b107ba2")

	if err := config.SetNostrNotificationConfig(true, nil, []cashu.WrappedPublicKey{npub1, npub2}); err != nil {
		t.Fatalf("config.SetNostrNotificationConfig(...): %v", err)
	}
	config.NOSTR_NOTIFICATION_NIP04_DM = true

	if err := db.UpdateNostrNotificationConfig(config); err != nil {
		t.Fatalf("db.UpdateNostrNotificationConfig(config): %v", err)
	}

	updatedConfig, err := db.GetConfig()
	if err != nil {
		t.Fatalf("db.GetConfig(): %v", err)
	}

	if !updatedConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected nostr notifications to be enabled")
	}

	if !updatedConfig.NOSTR_NOTIFICATION_NIP04_DM {
		t.Fatal("expected nostr notification NIP-04 DM flag to be enabled")
	}

	if len(updatedConfig.NOSTR_NOTIFICATION_NSEC) != 0 {
		t.Fatal("expected nostr notification nsec to stay out of database reads")
	}

	if len(updatedConfig.NOSTR_NOTIFICATION_NPUBS) != 2 {
		t.Fatalf("expected 2 stored npubs, got %d", len(updatedConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	if updatedConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex() != npub1.ToHex() {
		t.Fatalf("first stored npub mismatch: got %s want %s", updatedConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex(), npub1.ToHex())
	}

	if updatedConfig.NOSTR_NOTIFICATION_NPUBS[1].ToHex() != npub2.ToHex() {
		t.Fatalf("second stored npub mismatch: got %s want %s", updatedConfig.NOSTR_NOTIFICATION_NPUBS[1].ToHex(), npub2.ToHex())
	}
}

func TestUpdateConfig_PersistsNostrNotificationFields(t *testing.T) {
	db, _ := setupTestDB(t)

	var config utils.Config
	config.Default()

	if err := db.SetConfig(config); err != nil {
		t.Fatalf("db.SetConfig(config): %v", err)
	}

	npub := mustWrappedPubkey(t, "03d56ce4e446a85bbdaa547b4ec2b073d40ff802831352b8272b7dd7a4de5a7cac")
	nsec := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	if err := config.SetNostrNotificationConfig(true, nsec, []cashu.WrappedPublicKey{npub}); err != nil {
		t.Fatalf("config.SetNostrNotificationConfig(...): %v", err)
	}
	config.NOSTR_NOTIFICATION_NIP04_DM = true

	if err := db.UpdateConfig(config); err != nil {
		t.Fatalf("db.UpdateConfig(config): %v", err)
	}

	updatedConfig, err := db.GetConfig()
	if err != nil {
		t.Fatalf("db.GetConfig(): %v", err)
	}

	if !updatedConfig.NOSTR_NOTIFICATIONS {
		t.Fatal("expected nostr notifications to be enabled")
	}

	if !updatedConfig.NOSTR_NOTIFICATION_NIP04_DM {
		t.Fatal("expected nostr notification NIP-04 DM flag to be enabled")
	}

	if len(updatedConfig.NOSTR_NOTIFICATION_NSEC) != 0 {
		t.Fatal("expected nostr notification nsec to stay out of database reads")
	}

	if len(updatedConfig.NOSTR_NOTIFICATION_NPUBS) != 1 {
		t.Fatalf("expected 1 stored npub, got %d", len(updatedConfig.NOSTR_NOTIFICATION_NPUBS))
	}

	if updatedConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex() != npub.ToHex() {
		t.Fatalf("stored npub mismatch: got %s want %s", updatedConfig.NOSTR_NOTIFICATION_NPUBS[0].ToHex(), npub.ToHex())
	}
}
