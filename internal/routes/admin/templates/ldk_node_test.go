package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLdkPageShellRendersOnchainSection(t *testing.T) {
	var b bytes.Buffer
	if err := LdkPageShell(true, LdkSectionOnchain, LdkOnchainPageContent()).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPageShell(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"class=\"ldk-section-nav\"",
		"href=\"/admin/ldk\"",
		"href=\"/admin/ldk/lightning\"",
		"ldk-section-tab-active",
		"hx-get=\"/admin/ldk/onchain/balances\"",
		"hx-target=\"#ldk-balances-fragment\"",
		"hx-get=\"/admin/ldk/onchain/address\"",
		"hx-get=\"/admin/ldk/onchain/send-form\"",
		"hx-trigger=\"load\"",
		"hx-swap=\"outerHTML\"",
		"Generate on-chain address",
		"Send on-chain",
		"id=\"ldk-action-panel\"",
		"htmx-indicator",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in LDK page output", check)
		}
	}

	if strings.Count(out, "id=\"ldk-balances-fragment\"") != 1 {
		t.Fatalf("expected exactly one balances fragment shell")
	}
	if strings.Contains(out, "hx-get=\"/admin/ldk/lightning/network-summary\"") {
		t.Fatalf("did not expect lightning summary loader in on-chain shell")
	}
	if strings.Contains(out, "hx-get=\"/admin/ldk/lightning/channels\"") {
		t.Fatalf("did not expect lightning channels loader in on-chain shell")
	}
	if strings.Contains(out, "Open channel") {
		t.Fatalf("did not expect lightning action in on-chain shell")
	}
	if strings.Contains(out, "class=\"ldk-section-tab ldk-section-tab-active\" href=\"/admin/ldk/lightning\"") {
		t.Fatalf("did not expect lightning tab to be active")
	}
}

func TestLdkPageShellRendersLightningSection(t *testing.T) {
	var b bytes.Buffer
	if err := LdkPageShell(true, LdkSectionLightning, LdkLightningPageContent()).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPageShell(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"class=\"ldk-section-nav\"",
		"href=\"/admin/ldk\"",
		"href=\"/admin/ldk/lightning\"",
		"ldk-section-tab-active",
		"hx-get=\"/admin/ldk/lightning/network-summary\"",
		"hx-get=\"/admin/ldk/lightning/channel-form\"",
		"hx-get=\"/admin/ldk/lightning/channels\"",
		"Lightning balance",
		"Open channel",
		"id=\"ldk-channels-fragment\"",
		"id=\"ldk-action-panel\"",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in lightning shell output", check)
		}
	}

	if strings.Contains(out, "hx-get=\"/admin/ldk/onchain/balances\"") {
		t.Fatalf("did not expect on-chain balances loader in lightning shell")
	}
	if strings.Contains(out, "hx-get=\"/admin/ldk/onchain/address\"") {
		t.Fatalf("did not expect on-chain address action in lightning shell")
	}
	if strings.Contains(out, "Generate on-chain address") {
		t.Fatalf("did not expect on-chain action in lightning shell")
	}
}

func TestLdkPageShellRendersPaymentsSection(t *testing.T) {
	var b bytes.Buffer
	if err := LdkPageShell(true, LdkSectionPayments, LdkPaymentsPageContent()).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPageShell(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"class=\"ldk-section-nav\"",
		"href=\"/admin/ldk\"",
		"href=\"/admin/ldk/lightning\"",
		"href=\"/admin/ldk/payments\"",
		"id=\"ldk-payments-fragment\"",
		"hx-get=\"/admin/ldk/payments/list?type=all&show=25\"",
		"hx-trigger=\"load\"",
		"hx-target=\"#ldk-payments-fragment\"",
		"hx-swap=\"outerHTML\"",
		"Loading payments",
		"ldk-section-tab-active",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in payments shell output", check)
		}
	}

	if strings.Contains(out, "hx-get=\"/admin/ldk/onchain/balances\"") {
		t.Fatalf("did not expect on-chain balances loader in payments shell")
	}
	if strings.Contains(out, "hx-get=\"/admin/ldk/lightning/network-summary\"") {
		t.Fatalf("did not expect lightning summary loader in payments shell")
	}
	if strings.Contains(out, "hx-get=\"/admin/ldk/lightning/channels\"") {
		t.Fatalf("did not expect lightning channels loader in payments shell")
	}
	if strings.Contains(out, "Generate on-chain address") || strings.Contains(out, "Open channel") {
		t.Fatalf("did not expect on-chain or lightning actions in payments shell")
	}
	if strings.Contains(out, "class=\"ldk-section-tab ldk-section-tab-active\" href=\"/admin/ldk\"") {
		t.Fatalf("did not expect on-chain tab to be active")
	}
	if strings.Contains(out, "class=\"ldk-section-tab ldk-section-tab-active\" href=\"/admin/ldk/lightning\"") {
		t.Fatalf("did not expect lightning tab to be active")
	}
}

func TestLdkPaymentsFragmentRendersSuccessState(t *testing.T) {
	page := LdkPaymentsPage{
		ActiveFilter:          LdkPaymentsFilterIncoming,
		SelectedShow:          LdkPaymentsShow25,
		TotalItems:            2,
		ShowingFrom:           1,
		ShowingTo:             2,
		RetryQuery:            LdkPaymentsQuery(LdkPaymentsFilterAll, LdkPaymentsShow25),
		ShowOptions:           []LdkPaymentsShowOptionData{{Label: "25", Value: LdkPaymentsShow25, Query: LdkPaymentsQuery(LdkPaymentsFilterIncoming, LdkPaymentsShow25), Selected: true}, {Label: "ALL", Value: LdkPaymentsShowAll, Query: LdkPaymentsQuery(LdkPaymentsFilterIncoming, LdkPaymentsShowAll)}},
		CopyButtonClass:       "ldk-payment-copy-btn",
		CopyButtonDefaultText: "Copy",
		Rows: []LdkPaymentRow{{
			DirectionLabel:         "Inbound Payment",
			DirectionKey:           "inbound",
			KindBadgeLabel:         "ON-CHAIN",
			Amount:                 "100.000 sats",
			StatusLabel:            "Succeeded",
			IdentifierLabel:        "TRANSACTION ID",
			IdentifierValue:        "abcdef1234567890",
			ShortIdentifierValue:   "abcdef123456...",
			FormattedLastUpdatedAt: "2026-03-25 16:18:05 UTC",
			CopyPayload:            "abcdef1234567890",
			CanCopy:                true,
		}},
	}

	var b bytes.Buffer
	if err := LdkPaymentsFragment(page).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPaymentsFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-payments-fragment\"",
		"Payment history",
		"Showing 1 to 2 of 2 payments",
		">Incoming<",
		"Inbound Payment",
		"ON-CHAIN",
		"Succeeded",
		"TRANSACTION ID",
		"abcdef123456...",
		"2026-03-25 16:18:05 UTC",
		"data-copy-text=\"abcdef1234567890\"",
		"ldk-payment-copy-btn",
		">Show:</span>",
		">25</button>",
		">ALL</button>",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in payments success fragment", check)
		}
	}
}

func TestLdkPaymentsFragmentRendersEmptyState(t *testing.T) {
	page := LdkPaymentsPage{
		ActiveFilter:          LdkPaymentsFilterOutgoing,
		SelectedShow:          LdkPaymentsShow150,
		TotalItems:            0,
		ShowingFrom:           0,
		ShowingTo:             0,
		EmptyMessage:          "No outgoing payments found.",
		RetryQuery:            LdkPaymentsQuery(LdkPaymentsFilterAll, LdkPaymentsShow25),
		ShowOptions:           []LdkPaymentsShowOptionData{{Label: "150", Value: LdkPaymentsShow150, Query: LdkPaymentsQuery(LdkPaymentsFilterOutgoing, LdkPaymentsShow150), Selected: true}},
		CopyButtonClass:       "ldk-payment-copy-btn",
		CopyButtonDefaultText: "Copy",
	}

	var b bytes.Buffer
	if err := LdkPaymentsFragment(page).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPaymentsFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-payments-fragment\"",
		"Showing 0 to 0 of 0 payments",
		"No outgoing payments found.",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in payments empty fragment", check)
		}
	}
}

func TestLdkPaymentsFragmentRendersErrorState(t *testing.T) {
	page := LdkPaymentsPage{
		ErrorMessage:          "Invalid payments page",
		RetryQuery:            LdkPaymentsQuery(LdkPaymentsFilterAll, LdkPaymentsShow25),
		CopyButtonClass:       "ldk-payment-copy-btn",
		CopyButtonDefaultText: "Copy",
	}

	var b bytes.Buffer
	if err := LdkPaymentsFragment(page).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkPaymentsFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-payments-fragment\"",
		"Invalid payments page",
		"Retry",
		"hx-get=\"/admin/ldk/payments/list?type=all&amp;show=25\"",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in payments error fragment", check)
		}
	}
	if strings.Contains(out, "Showing ") {
		t.Fatalf("did not expect summary text in error fragment")
	}
}

func TestLdkBalancesFragmentRendersOnchainOnlyCard(t *testing.T) {
	var b bytes.Buffer
	if err := LdkBalancesFragment("1.000 sats", "900 sats").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkBalancesFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-balances-fragment\"",
		"id=\"ldk-balances-card\"",
		"id=\"ldk-total-onchain-balance\"",
		"id=\"ldk-available-onchain-balance\"",
		"class=\"ldk-summary-grid\"",
		"Total On-chain",
		"Available On-chain",
		"1.000",
		"900",
		"ldk-amount-unit",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in balances fragment", check)
		}
	}
	if strings.Contains(out, "Lightning balance") {
		t.Fatalf("did not expect lightning balance label in on-chain balances fragment")
	}
}

func TestLdkBalancesErrorFragmentUsesStableRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkBalancesErrorFragment("Could not load LDK balances").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkBalancesErrorFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-balances-fragment\"",
		"id=\"ldk-balances-card\"",
		"Balances unavailable",
		"Could not load LDK balances",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in balances error fragment", check)
		}
	}
}

func TestLdkNetworkSummaryFragmentRendersCounts(t *testing.T) {
	var b bytes.Buffer
	if err := LdkNetworkSummaryFragment("2.000 sats", 2, 2, 3, 1).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkNetworkSummaryFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-network-summary-fragment\"",
		"Lightning balance",
		"2.000",
		"Connected Peers",
		"Channels",
		"2 / 2",
		"1 / 3",
		"active / total",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in network summary fragment", check)
		}
	}
}

func TestLdkOnchainSendFormFragmentUsesActionPanelRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkOnchainSendFormFragment(9500).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkOnchainSendFormFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-action-panel\"",
		"Send on-chain",
		"Bitcoin address",
		"/admin/ldk/onchain/send",
		"hx-target=\"#ldk-action-panel\"",
		"hx-disabled-elt=\"this\"",
		"9500 sats",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in on-chain send form fragment", check)
		}
	}
}

func TestLdkOnchainSendSubmittedOOBFragmentUsesActionPanelRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkOnchainSendSubmittedOOBFragment().Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkOnchainSendSubmittedOOBFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-action-panel\" hx-swap-oob=\"outerHTML\"",
		"Payment submitted. Reopen this form to send another payment.",
		"Payment sent",
		"disabled",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in on-chain send submitted fragment", check)
		}
	}
}

func TestLdkNetworkSummaryErrorFragmentUsesStableRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkNetworkSummaryErrorFragment("Could not load network summary").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkNetworkSummaryErrorFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-network-summary-fragment\"",
		"Network summary unavailable",
		"Could not load network summary",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in network summary error fragment", check)
		}
	}
}

func TestLdkAddressFragmentUsesActionPanelRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkAddressFragment("bc1example", "base64png").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkAddressFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-action-panel\"",
		"On-chain address",
		"bc1example",
		"Copy address",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in address fragment", check)
		}
	}
	if strings.Contains(out, "Generate a deposit address for funding your node.") {
		t.Fatalf("did not expect legacy address subtitle in address fragment")
	}
}

func TestLdkActionPanelErrorFragmentUsesStableRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkActionPanelErrorFragment("Could not load channel form").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkActionPanelErrorFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-action-panel\"",
		"Could not load channel form",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in action panel error fragment", check)
		}
	}
}

func TestLdkOpenChannelFormFragmentUsesActionPanelRoot(t *testing.T) {
	var b bytes.Buffer
	if err := LdkOpenChannelFormFragment(9500).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkOpenChannelFormFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-action-panel\"",
		"Open channel",
		"/admin/ldk/lightning/channels/open",
		"hx-target=\"#ldk-channels-fragment\"",
		"9500 sats",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in open channel fragment", check)
		}
	}
}

func TestLdkChannelsFragmentRendersPersistentRows(t *testing.T) {
	rows := []LdkChannelRow{{
		ChannelID:         "chan-1",
		CounterpartyLabel: "peer-a",
		CounterpartyPub:   "0211",
		LocalBalance:      "60 sats",
		RemoteBalance:     "40 sats",
		LocalBalanceSats:  60,
		RemoteBalanceSats: 40,
		TotalBalanceSats:  100,
		LocalBalancePct:   60,
		RemoteBalancePct:  40,
		StateLabel:        "Active",
		CanClose:          true,
	}}

	var b bytes.Buffer
	if err := LdkChannelsFragment(rows).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkChannelsFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-channels-fragment\"",
		"Lightning channels",
		"peer-a",
		"0211",
		"Local balance",
		"60",
		"Remote balance",
		"40",
		"ldk-channel-balance-bar",
		"width:60%",
		"60% local",
		"40% remote",
		"aria-label=\"Close channel\"",
		"title=\"Close channel\"",
		"Force close",
	}
	for _, check := range checks {
		if strings.Contains(check, "Force close") {
			if strings.Contains(out, check) {
				t.Fatalf("did not expect force close action for cooperative-close-only row")
			}
			continue
		}
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in channels fragment", check)
		}
	}
	if strings.Contains(out, "<details") || strings.Contains(out, "<summary") || strings.Contains(out, "Details") {
		t.Fatalf("did not expect expandable channel markup in channels fragment")
	}
	if strings.Contains(out, "Track channel state and balance distribution at a glance.") {
		t.Fatalf("did not expect channels subtitle in channels fragment")
	}
}

func TestLdkChannelsFragmentRendersForceCloseTitle(t *testing.T) {
	rows := []LdkChannelRow{{
		ChannelID:         "0222",
		CounterpartyPub:   "03aa",
		CounterpartyLabel: "peer-b",
		StateLabel:        "Offline",
		LocalBalancePct:   20,
		RemoteBalancePct:  80,
		CanForceClose:     true,
	}}

	var b bytes.Buffer
	if err := LdkChannelsFragment(rows).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkChannelsFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"Force close",
		"title=\"Force close channel\"",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in force close channels fragment", check)
		}
	}
}

func TestLdkChannelsFragmentEmptyState(t *testing.T) {
	var b bytes.Buffer
	if err := LdkChannelsFragment(nil).Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkChannelsFragment(nil).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-channels-fragment\"",
		"Lightning channels",
		"No channels are currently open.",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in channels empty state", check)
		}
	}
}

func TestLdkChannelsErrorFragmentPreservesHeading(t *testing.T) {
	var b bytes.Buffer
	if err := LdkChannelsErrorFragment("Could not load channels").Render(context.Background(), &b); err != nil {
		t.Fatalf("LdkChannelsErrorFragment(...).Render: %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-channels-fragment\"",
		"Lightning channels",
		"Could not load channels",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in channels error fragment", check)
		}
	}
}
