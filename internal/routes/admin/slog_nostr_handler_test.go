package admin

import (
	"io"
	"log/slog"
	"testing"
	"time"

	m "github.com/lescuer97/nutmix/internal/mint"
)

func TestFormatRecordForNostrIncludesSortedAttrs(t *testing.T) {
	record := slog.NewRecord(time.Now(), slog.LevelError, "boom", 0)
	record.AddAttrs(slog.String("z", "last"), slog.String("a", "first"))

	formatted := formatRecordForNostr(record)
	want := "[ERROR] boom | a=first z=last"
	if formatted != want {
		t.Fatalf("formatRecordForNostr(record): got %q want %q", formatted, want)
	}
}

func TestNewNostrErrorNotifyHandlerCreatesHandlerWhenNotificationsDisabled(t *testing.T) {
	base := slog.NewTextHandler(io.Discard, nil)
	var mintValue m.Mint
	mintValue.Config.NOSTR_NOTIFICATIONS = false

	h := NewNostrErrorNotifyHandler(base, &mintValue)
	if h == nil {
		t.Fatal("expected handler even when notifications are disabled")
	}
}

func TestNewNostrErrorNotifyHandlerCreatesHandlerWhenNip04DmDisabled(t *testing.T) {
	base := slog.NewTextHandler(io.Discard, nil)
	var mintValue m.Mint
	mintValue.Config.NOSTR_NOTIFICATIONS = true
	mintValue.Config.NOSTR_NOTIFICATION_NIP04_DM = false

	h := NewNostrErrorNotifyHandler(base, &mintValue)
	if h == nil {
		t.Fatal("expected handler even when NIP-04 DM notifications are disabled")
	}
}
