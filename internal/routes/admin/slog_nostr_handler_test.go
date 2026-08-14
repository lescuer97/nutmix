package admin

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

type recordingSlogHandler struct {
	err     error
	records []slog.Record
}

func (h *recordingSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingSlogHandler) Handle(_ context.Context, record slog.Record) error {
	h.records = append(h.records, record.Clone())
	return h.err
}

func (h *recordingSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func TestNotificationPayloadAllowsOnlyKnownErrorEvent(t *testing.T) {
	fixedTime := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	record := slog.NewRecord(fixedTime, slog.LevelError, "CANARY_MESSAGE", 0)
	record.AddAttrs(
		slog.String(nostrNotificationEventAttr, nostrNotificationEventTest),
		slog.String("proofs", "CANARY_PROOF_SECRET"),
		slog.String("error", "CANARY_INTERNAL_ERROR"),
		slog.Group("nested", slog.String("secret", "CANARY_NESTED_SECRET")),
	)

	payload, ok := notificationPayload(record)
	if !ok {
		t.Fatal("expected allow-listed error event to produce a notification")
	}
	want := "[ERROR] event=test occurred_at=2026-08-17T12:00:00Z"
	if payload != want {
		t.Fatalf("notificationPayload(record): got %q want %q", payload, want)
	}
}

func TestNotificationPayloadRejectsUnapprovedRecords(t *testing.T) {
	fixedTime := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		attrs []slog.Attr
		level slog.Level
	}{
		{name: "missing marker", level: slog.LevelError, attrs: nil},
		{name: "warning", level: slog.LevelWarn, attrs: []slog.Attr{slog.String(nostrNotificationEventAttr, nostrNotificationEventTest)}},
		{name: "unknown event", level: slog.LevelError, attrs: []slog.Attr{slog.String(nostrNotificationEventAttr, "unknown")}},
		{name: "empty event", level: slog.LevelError, attrs: []slog.Attr{slog.String(nostrNotificationEventAttr, "")}},
		{name: "non string event", level: slog.LevelError, attrs: []slog.Attr{slog.Bool(nostrNotificationEventAttr, true)}},
		{name: "duplicate marker", level: slog.LevelError, attrs: []slog.Attr{
			slog.String(nostrNotificationEventAttr, nostrNotificationEventTest),
			slog.String(nostrNotificationEventAttr, nostrNotificationEventTest),
		}},
		{name: "nested marker", level: slog.LevelError, attrs: []slog.Attr{
			slog.Group("nested", slog.String(nostrNotificationEventAttr, nostrNotificationEventTest)),
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := slog.NewRecord(fixedTime, test.level, "CANARY_MESSAGE", 0)
			record.AddAttrs(test.attrs...)
			if payload, ok := notificationPayload(record); ok {
				t.Fatalf("notificationPayload(record): unexpectedly allowed payload %q", payload)
			}
		})
	}
}

func TestNostrErrorNotifyHandlerPreservesFullBaseRecord(t *testing.T) {
	baseErr := errors.New("base handler error")
	base := &recordingSlogHandler{err: baseErr, records: nil}
	h := NewNostrErrorNotifyHandler(base, nil)
	fixedTime := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	record := slog.NewRecord(fixedTime, slog.LevelError, "CANARY_FULL_LOCAL_MESSAGE", 0)
	record.AddAttrs(
		slog.String("proofs", "CANARY_PROOF_SECRET"),
		slog.String("error", "CANARY_INTERNAL_ERROR"),
	)

	err := h.Handle(context.Background(), record)
	if !errors.Is(err, baseErr) {
		t.Fatalf("Handle returned %v, want base handler error %v", err, baseErr)
	}
	if len(base.records) != 1 {
		t.Fatalf("base handler received %d records, want 1", len(base.records))
	}

	got := base.records[0]
	if got.Message != record.Message || got.Level != record.Level || !got.Time.Equal(record.Time) {
		t.Fatalf("base record metadata changed: got message=%q level=%v time=%v", got.Message, got.Level, got.Time)
	}
	attrs := make(map[string]string)
	got.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.String()
		return true
	})
	if attrs["proofs"] != "CANARY_PROOF_SECRET" || attrs["error"] != "CANARY_INTERNAL_ERROR" {
		t.Fatalf("base record attributes changed: %v", attrs)
	}
}

func TestNewNostrErrorNotifyHandlerCreatesHandlerWhenNotificationsDisabled(t *testing.T) {
	base := slog.DiscardHandler
	var mintValue m.Mint
	var nostrNotificationConfig utils.NostrNotificationConfig
	mintValue.NostrNotificationConfig = &nostrNotificationConfig

	h := NewNostrErrorNotifyHandler(base, &mintValue)
	if h == nil {
		t.Fatal("expected handler even when notifications are disabled")
	}
}

func TestNewNostrErrorNotifyHandlerCreatesHandlerWhenNip04DmDisabled(t *testing.T) {
	base := slog.DiscardHandler
	var mintValue m.Mint
	var nostrNotificationConfig utils.NostrNotificationConfig
	nostrNotificationConfig.NOSTR_NOTIFICATIONS = true
	mintValue.NostrNotificationConfig = &nostrNotificationConfig

	h := NewNostrErrorNotifyHandler(base, &mintValue)
	if h == nil {
		t.Fatal("expected handler even when NIP-04 DM notifications are disabled")
	}
}
