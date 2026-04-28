package admin

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/keyer"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip17"
)

var nostrDefaultDMRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://relay.primal.net",
}

var bluePagesRelays = []string{
	"wss://purplepag.es",
	"wss://index.hzrd149.com",
}

const nostrDMPublishTimeout = 8 * time.Second

type NostrErrorNotifyHandler struct {
	base slog.Handler
	mint *m.Mint
	pool *nostr.SimplePool
	key  nostr.Keyer
}

func NewNostrErrorNotifyHandler(base slog.Handler, mint *m.Mint) *NostrErrorNotifyHandler {
	return &NostrErrorNotifyHandler{
		base: base,
		mint: mint,
		pool: nil,
		key:  nil,
	}
}

func (h *NostrErrorNotifyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *NostrErrorNotifyHandler) Handle(ctx context.Context, record slog.Record) error {
	handleErr := h.base.Handle(ctx, record)

	if record.Level < slog.LevelError || h.mint == nil {
		return handleErr
	}

	if h.mint.NostrNotificationConfig == nil || !h.mint.NostrNotificationConfig.NOSTR_NOTIFICATIONS {
		return handleErr
	}

	message := formatRecordForNostr(record)
	if strings.TrimSpace(message) == "" {
		return handleErr
	}

	go func(parentCtx context.Context, payload string) {
		ctx, cancel := context.WithTimeout(parentCtx, 7*time.Second)
		defer cancel()
		for _, pubkey := range h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NPUBS {
			_ = h.SendPrivateNostrMessage(ctx, pubkey, payload)
		}
	}(ctx, message)

	return handleErr
}

func (h *NostrErrorNotifyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &NostrErrorNotifyHandler{
		base: h.base.WithAttrs(attrs),
		mint: h.mint,
		pool: h.pool,
		key:  h.key,
	}
}

func (h *NostrErrorNotifyHandler) WithGroup(name string) slog.Handler {
	return &NostrErrorNotifyHandler{
		base: h.base.WithGroup(name),
		mint: h.mint,
		pool: h.pool,
		key:  h.key,
	}
}

func (h *NostrErrorNotifyHandler) verifyNostrKeys(ctx context.Context, nsec []byte) error {
	if h.pool == nil {
		newPool := nostr.NewSimplePool(ctx)
		h.pool = newPool
	}
	if h.key == nil {
		signer, err := keyer.NewPlainKeySigner(hex.EncodeToString(nsec))
		if err != nil {
			return fmt.Errorf("keyer.NewPlainKeySigner(hex.EncodeToString(nsec)). %w", err)
		}
		h.key = signer
	}
	return nil
}
func (h *NostrErrorNotifyHandler) SendPrivateNostrMessage(ctx context.Context, recepientPubKey cashu.WrappedPublicKey, message string) error {
	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" {
		return nil
	}
	if h.mint.NostrNotificationConfig == nil || !h.mint.NostrNotificationConfig.NOSTR_NOTIFICATIONS || h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NSEC == nil {
		return nil
	}
	err := h.verifyNostrKeys(ctx, h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NSEC)
	if err != nil {
		return fmt.Errorf("h.verifyNostrKeys(ctx, h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NSEC). %w", err)
	}

	recipientCtx, cancel := context.WithTimeout(ctx, nostrDMPublishTimeout)
	defer cancel()
	recipientHexPubKey := hex.EncodeToString(schnorr.SerializePubKey(recepientPubKey.PublicKey))

	relays := nip17.GetDMRelays(recipientCtx, recipientHexPubKey, h.pool, bluePagesRelays)
	relays = append(relays, nostrDefaultDMRelays...)

	if h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NIP04_DM {
		err = h.sendNIP04PrivateNostrMessage(recipientCtx, recipientHexPubKey, relays, trimmedMessage)
		if err != nil {
			return fmt.Errorf("h.sendNIP04PrivateNostrMessage(recipientCtx, recipientHexPubKey, relays, trimmedMessage). %w", err)
		}
		return nil
	}

	err = h.sendNIP17PrivateNostrMessage(recipientCtx, recipientHexPubKey, relays, trimmedMessage)
	if err != nil {
		return fmt.Errorf("h.sendNIP17PrivateNostrMessage(recipientCtx, recipientHexPubKey, relays, trimmedMessage). %w", err)
	}

	return nil
}

func (h *NostrErrorNotifyHandler) sendNIP04PrivateNostrMessage(ctx context.Context, recipientHexPubKey string, relays []string, message string) error {
	sharedSecret, err := nip04.ComputeSharedSecret(recipientHexPubKey, hex.EncodeToString(h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NSEC))
	if err != nil {
		return fmt.Errorf("nip04.ComputeSharedSecret(recipientHexPubKey, hex.EncodeToString(h.mint.NostrNotificationConfig.NOSTR_NOTIFICATION_NSEC)). %w", err)
	}

	encryptedContent, err := nip04.Encrypt(message, sharedSecret)
	if err != nil {
		return fmt.Errorf("nip04.Encrypt(message, sharedSecret). %w", err)
	}

	event := nostr.Event{
		Kind:      nostr.KindEncryptedDirectMessage,
		CreatedAt: nostr.Now(),
		Tags:      nostr.Tags{nostr.Tag{"p", recipientHexPubKey}},
		Content:   encryptedContent,
		ID:        "",
		PubKey:    "",
		Sig:       "",
	}
	if err := h.key.SignEvent(ctx, &event); err != nil {
		return fmt.Errorf("h.key.SignEvent(ctx, &event). %w", err)
	}

	results := h.pool.PublishMany(ctx, relays, event)
	publishSuccess := false
	var publishErr error
	for result := range results {
		if result.Error == nil {
			publishSuccess = true
		}
		if result.Error != nil {
			publishErr = result.Error
		}
	}

	if !publishSuccess && publishErr != nil {
		return fmt.Errorf("h.pool.PublishMany(ctx, relays, event). %w", publishErr)
	}

	return nil
}

func (h *NostrErrorNotifyHandler) sendNIP17PrivateNostrMessage(ctx context.Context, recipientHexPubKey string, relays []string, message string) error {
	err := nip17.PublishMessage(
		ctx,
		message,
		nostr.Tags{},
		h.pool,
		nostrDefaultDMRelays,
		relays,
		h.key,
		recipientHexPubKey,
		nil,
	)
	if err != nil {
		return fmt.Errorf("nip17.PublishMessage(...). %w", err)
	}

	return nil
}

func formatRecordForNostr(record slog.Record) string {
	attrs := make([]string, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%s", attr.Key, attr.Value.String()))
		return true
	})
	sort.Strings(attrs)

	if len(attrs) == 0 {
		return fmt.Sprintf("[%s] %s", record.Level.String(), record.Message)
	}

	return fmt.Sprintf("[%s] %s | %s", record.Level.String(), record.Message, strings.Join(attrs, " "))
}
