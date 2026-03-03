package templates

import (
	"encoding/hex"
	"log/slog"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// formatNumber formats a number with thousand separators (periods)
func FormatNumber(n uint64) string {
	p := message.NewPrinter(language.German)
	return p.Sprintf("%.0f", float64(n))
}

func NostrNotificationServiceNpub(nsec []byte) string {
	if len(nsec) == 0 {
		return ""
	}

	privateKeyHex := hex.EncodeToString(nsec)
	publicKeyHex, err := nostr.GetPublicKey(privateKeyHex)
	if err != nil {
		slog.Warn("nostr.GetPublicKey(privateKeyHex)", slog.Any("error", err))
		return ""
	}

	npub, err := nip19.EncodePublicKey(publicKeyHex)
	if err != nil {
		slog.Warn("nip19.EncodePublicKey(publicKeyHex)", slog.Any("error", err))
		return ""
	}

	return npub
}

func WrappedPublicKeyToNpub(pubkey cashu.WrappedPublicKey) string {
	if pubkey.PublicKey == nil {
		return ""
	}

	publicKeyHex := hex.EncodeToString(schnorr.SerializePubKey(pubkey.PublicKey))
	npub, err := nip19.EncodePublicKey(publicKeyHex)
	if err != nil {
		slog.Warn("nip19.EncodePublicKey(publicKeyHex)", slog.Any("error", err))
		return ""
	}

	return npub
}
