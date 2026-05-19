package admin

import (
	"log/slog"
	"testing"
	"time"

	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
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
