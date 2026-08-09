//nolint:exhaustruct
package admin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestLightningBackendStatusComponent(t *testing.T) {
	tests := []struct {
		name   string
		status lightning.NodeStatus
		text   string
		class  string
		icon   string

		deprecated bool
	}{
		{name: "online", status: lightning.ONLINE_STATUS, text: "Online", class: "lightning-status-online", icon: "m9 12 2 2 4-4"},
		{name: "offline", status: lightning.OFFLINE_STATUS, text: "Offline", class: "lightning-status-offline", icon: "m15 9-6 6"},
		{name: "stopped", status: lightning.STOPPED_STATUS, text: "Stopped", class: "lightning-status-stopped", icon: `width="12" height="12"`},
		{name: "unknown", status: lightning.UNKNOWN_STATUS, text: "Unknown", class: "lightning-status-unknown", icon: "M9.09 9"},
		{name: "online deprecated", status: lightning.ONLINE_STATUS, deprecated: true, text: "Online", class: "lightning-status-online", icon: "m9 12 2 2 4-4"},
		{name: "offline deprecated", status: lightning.OFFLINE_STATUS, deprecated: true, text: "Offline", class: "lightning-status-offline", icon: "m15 9-6 6"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, recorder := adminTestContext("/admin/lightning-status")
			if err := templates.LightningBackendStatus(test.status, test.deprecated).Render(ctx.Request.Context(), recorder); err != nil {
				t.Fatalf("Render: %v", err)
			}

			body := recorder.Body.String()
			for _, want := range []string{test.text, test.class, test.icon, `id="lightning-status-loading"`} {
				if !strings.Contains(body, want) {
					t.Fatalf("expected body to contain %q, got %s", want, body)
				}
			}
			if strings.Contains(body, "Check again") {
				t.Fatalf("status component must not include a manual refresh button, got %s", body)
			}
			badge := `<span class="lightning-status-deprecation-badge">Deprecated</span>`
			if strings.Contains(body, badge) != test.deprecated {
				t.Fatalf("expected deprecated badge presence to be %t, got %s", test.deprecated, body)
			}
			if test.deprecated && strings.Index(body, test.text) > strings.Index(body, badge) {
				t.Fatalf("expected connectivity to render before deprecation, got %s", body)
			}
			for _, unwanted := range []string{"lightning-status-depracated", "m21.73 18"} {
				if strings.Contains(body, unwanted) {
					t.Fatalf("did not expect deprecated health behavior %q, got %s", unwanted, body)
				}
			}
		})
	}
}

func TestLightningBackendStatusLoader(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln")
	if err := templates.LightningBackendStatusLoader().Render(ctx.Request.Context(), recorder); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	for _, want := range []string{`class="card card-md lightning-status-panel"`, `role="status"`, `aria-live="polite"`, `hx-trigger="load, every 30s, lightning-status-changed from:body"`, `hx-sync="#lightning-backend-status:replace"`, "Checking connection"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
}

func TestLightningActivityLayoutPlacesStatusBesideRangeSelector(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln")
	if err := templates.LightningActivityLayout(utils.Config{}, "1w", "", false, false, "").Render(ctx.Request.Context(), recorder); err != nil {
		t.Fatalf("Render: %v", err)
	}

	body := recorder.Body.String()
	toolbarStart := strings.Index(body, `class="lightning-activity-toolbar"`)
	selectorStart := strings.Index(body, `class="time-range-container"`)
	statusStart := strings.Index(body, `class="card card-md lightning-status-panel"`)
	chartStart := strings.Index(body, `id="ln-chart-placeholder"`)
	if toolbarStart == -1 || selectorStart < toolbarStart || statusStart < selectorStart || chartStart < statusStart {
		t.Fatalf("expected range selector and status in the top toolbar, got %s", body)
	}
}

type statusBackend struct {
	statusFunc func(context.Context) (lightning.NodeStatus, error)
	err        error
	status     lightning.NodeStatus
	lightning.FakeWallet
	backendType lightning.Backend
}

func (b statusBackend) Status(ctx context.Context) (lightning.NodeStatus, error) {
	if b.statusFunc != nil {
		return b.statusFunc(ctx)
	}
	return b.status, b.err
}

func (b statusBackend) LightningType() lightning.Backend {
	return b.backendType
}

func TestLightningBackendStatusHandler(t *testing.T) {
	tests := []struct {
		name     string
		backend  lightning.LightningBackend
		want     []string
		unwanted []string
	}{
		{name: "healthy LNBITS is deprecated", backend: statusBackend{backendType: lightning.LNBITS, status: lightning.ONLINE_STATUS}, want: []string{"Online", "lightning-status-online", "lightning-status-deprecation-badge", "Deprecated"}}, //nolint:staticcheck // Verify supported deprecated backend UI.
		{name: "Strike is stopped", backend: lightning.Strike{}, want: []string{"Stopped", "lightning-status-stopped"}, unwanted: []string{"lightning-status-deprecation-badge", "Deprecated", "Offline"}},
		{name: "offline deprecated backend", backend: statusBackend{backendType: lightning.LNBITS, status: lightning.OFFLINE_STATUS}, want: []string{"Offline", "lightning-status-offline", "lightning-status-deprecation-badge", "Deprecated"}}, //nolint:staticcheck // Verify supported deprecated backend UI.
		{name: "LND is not deprecated", backend: statusBackend{backendType: lightning.LNDGRPC, status: lightning.ONLINE_STATUS}, want: []string{"Online"}, unwanted: []string{"lightning-status-deprecation-badge", "Deprecated"}},
		{name: "CLN is not deprecated", backend: statusBackend{backendType: lightning.CLNGRPC, status: lightning.OFFLINE_STATUS}, want: []string{"Offline"}, unwanted: []string{"lightning-status-deprecation-badge", "Deprecated"}},
		{name: "fake is not deprecated", backend: statusBackend{backendType: lightning.FAKEWALLET, status: lightning.ONLINE_STATUS}, want: []string{"Online"}, unwanted: []string{"lightning-status-deprecation-badge", "Deprecated"}},
		{name: "unknown", backend: statusBackend{backendType: lightning.FAKEWALLET, status: lightning.UNKNOWN_STATUS}, want: []string{"Unknown"}},
		{name: "nil backend", backend: nil, want: []string{"Unknown"}},
		{name: "unexpected status", backend: statusBackend{backendType: lightning.FAKEWALLET, status: lightning.NodeStatus("BROKEN")}, want: []string{"Unknown"}, unwanted: []string{"lightning-status-broken"}},
		{name: "backend error", backend: statusBackend{backendType: lightning.FAKEWALLET, status: lightning.UNKNOWN_STATUS, err: errors.New("rpc secret must stay private")}, want: []string{"Offline"}, unwanted: []string{"rpc secret must stay private"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mint := adminTestMint(testMockDB())
			mint.LightningBackend = test.backend
			ctx, recorder := adminTestContext("/admin/lightning-status")

			LightningBackendStatus(mint)(ctx)

			if recorder.Code != 200 {
				t.Fatalf("expected status 200, got %d", recorder.Code)
			}
			body := recorder.Body.String()
			for _, want := range test.want {
				if !strings.Contains(body, want) {
					t.Fatalf("expected body to contain %q, got %s", want, body)
				}
			}
			for _, unwanted := range test.unwanted {
				if strings.Contains(body, unwanted) {
					t.Fatalf("did not expect body to contain %q, got %s", unwanted, body)
				}
			}
		})
	}
}

func TestStrikeEndOfLifeAdminUI(t *testing.T) {
	ctx, recorder := adminTestContext("/admin/ln")
	config := utils.Config{MINT_LIGHTNING_BACKEND: utils.Strike} //nolint:exhaustruct,staticcheck // Verify legacy Strike UI.
	if err := templates.LightningBackendPage(config, true, false, "").Render(ctx.Request.Context(), recorder); err != nil {
		t.Fatalf("Render Strike page: %v", err)
	}
	body := recorder.Body.String()
	for _, want := range []string{`id="backend-end-of-life-alert"`, "Strike backend stopped", "select another Lightning backend"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
	for _, unwanted := range []string{`value="Strike"`, "STRIKE_KEY", "STRIKE_ENDPOINT"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect body to contain %q, got %s", unwanted, body)
		}
	}

	ctx, recorder = adminTestContext("/admin/ln")
	config.MINT_LIGHTNING_BACKEND = utils.FAKE_WALLET
	if err := templates.LightningBackendPage(config, false, false, "").Render(ctx.Request.Context(), recorder); err != nil {
		t.Fatalf("Render Fake Wallet page: %v", err)
	}
	if strings.Contains(recorder.Body.String(), `id="backend-end-of-life-alert"`) {
		t.Fatal("Strike alert rendered for a supported backend")
	}

	ctx, recorder = adminTestContext("/admin/lightningdata")
	if err := templates.SetupForms(string(utils.Strike), config, templates.DefaultLDKResourceSnapshot(), templates.LDKFormValues{}).Render(ctx.Request.Context(), recorder); err != nil { //nolint:staticcheck // Verify retired backend fields stay hidden.
		t.Fatalf("Render SetupForms: %v", err)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("Strike setup fields rendered: %s", recorder.Body.String())
	}
}

func TestLightningBackendStatusHandlerSetsFiveSecondDeadline(t *testing.T) {
	var receivedDeadline time.Time
	backend := statusBackend{
		backendType: lightning.FAKEWALLET,
		statusFunc: func(ctx context.Context) (lightning.NodeStatus, error) {
			var ok bool
			receivedDeadline, ok = ctx.Deadline()
			if !ok {
				t.Fatal("expected status context to have a deadline")
			}
			return lightning.ONLINE_STATUS, nil
		},
	}
	mint := adminTestMint(testMockDB())
	mint.LightningBackend = backend
	ctx, _ := adminTestContext("/admin/lightning-status")
	started := time.Now()

	LightningBackendStatus(mint)(ctx)

	timeout := receivedDeadline.Sub(started)
	if timeout < 4500*time.Millisecond || timeout > lightningStatusTimeout+100*time.Millisecond {
		t.Fatalf("expected deadline about five seconds after handler entry, got %v", timeout)
	}
}

func TestLightningBackendStatusHandlerPropagatesRequestCancellation(t *testing.T) {
	statusStarted := make(chan struct{})
	cancellation := make(chan error, 1)
	backend := statusBackend{
		backendType: lightning.FAKEWALLET,
		statusFunc: func(ctx context.Context) (lightning.NodeStatus, error) {
			close(statusStarted)
			<-ctx.Done()
			cancellation <- ctx.Err()
			return lightning.OFFLINE_STATUS, ctx.Err()
		},
	}
	mint := adminTestMint(testMockDB())
	mint.LightningBackend = backend
	ctx, _ := adminTestContext("/admin/lightning-status")
	requestCtx, cancel := context.WithCancel(ctx.Request.Context())
	ctx.Request = ctx.Request.WithContext(requestCtx)
	t.Cleanup(cancel)
	handlerDone := make(chan struct{})

	go func() {
		LightningBackendStatus(mint)(ctx)
		close(handlerDone)
	}()

	select {
	case <-statusStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("backend status check did not start")
	}
	cancel()

	select {
	case err := <-cancellation:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("request cancellation did not reach backend")
	}

	select {
	case <-handlerDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not return after request cancellation")
	}
}
