package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLayoutErrorToShowRendersWhenMessageSet(t *testing.T) {
	var b bytes.Buffer
	if err := LayoutErrorToShow("could not Create ldk-node. boom").Render(context.Background(), &b); err != nil {
		t.Fatalf("LayoutErrorToShow(...).Render: %v", err)
	}

	out := b.String()
	for _, want := range []string{"ldk-outage-banner", "minting and melting are offline", "could not Create ldk-node. boom", "/admin/settings"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected alert to contain %q, got %q", want, out)
		}
	}
}

func TestLayoutErrorToShowHiddenWhenMessageEmpty(t *testing.T) {
	var b bytes.Buffer
	if err := LayoutErrorToShow("").Render(context.Background(), &b); err != nil {
		t.Fatalf("LayoutErrorToShow(...).Render: %v", err)
	}

	if strings.Contains(b.String(), "ldk-outage-banner") {
		t.Fatalf("expected no banner for empty message, got %q", b.String())
	}
}

func TestLayoutIncludesLayoutErrorToShow(t *testing.T) {
	var b bytes.Buffer
	component := Layout("settings", false, false, "ldk is down")
	if err := component.Render(context.Background(), &b); err != nil {
		t.Fatalf("Layout(...).Render: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "ldk-outage-banner") || !strings.Contains(out, "ldk is down") {
		t.Fatalf("expected layout to render the ldk outage banner, got %q", out)
	}
}
