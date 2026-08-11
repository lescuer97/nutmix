package admin

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func TestMapLdkChannelRows(t *testing.T) {
	rows := mapLdkChannelRows([]ldk.LDKChannelSummary{
		{
			ChannelID:         "chan-active",
			State:             "active",
			PeerConnected:     true,
			CounterpartyLabel: "peer-a",
			CounterpartyPub:   "0211",
			LocalBalanceSats:  12345,
			RemoteBalanceSats: 45000,
		},
		{
			ChannelID:         "chan-offline",
			State:             "offline",
			PeerConnected:     false,
			CounterpartyLabel: "peer-b",
			CounterpartyPub:   "0222",
			LocalBalanceSats:  5,
			RemoteBalanceSats: 6,
		},
	})

	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}

	if rows[0].ChannelID != "chan-active" || rows[0].CounterpartyLabel != "peer-a" || rows[0].CounterpartyPub != "0211" {
		t.Fatalf("unexpected first row identity: %+v", rows[0])
	}
	if rows[0].LocalBalance != "12.345 sats" || rows[0].RemoteBalance != "45.000 sats" {
		t.Fatalf("unexpected active row balances: %+v", rows[0])
	}
	if rows[0].LocalBalanceSats != 12345 || rows[0].RemoteBalanceSats != 45000 || rows[0].TotalBalanceSats != 57345 {
		t.Fatalf("unexpected active row numeric balances: %+v", rows[0])
	}
	if rows[0].LocalBalancePct != 21 || rows[0].RemoteBalancePct != 79 {
		t.Fatalf("unexpected active row percentages: %+v", rows[0])
	}
	if rows[0].StateLabel != "Active" || !rows[0].CanClose || rows[0].CanForceClose {
		t.Fatalf("unexpected active row flags: %+v", rows[0])
	}

	if rows[1].LocalBalanceSats != 5 || rows[1].RemoteBalanceSats != 6 || rows[1].TotalBalanceSats != 11 {
		t.Fatalf("unexpected offline row numeric balances: %+v", rows[1])
	}
	if rows[1].LocalBalancePct != 45 || rows[1].RemoteBalancePct != 55 {
		t.Fatalf("unexpected offline row percentages: %+v", rows[1])
	}
	if rows[1].StateLabel != "Offline" || rows[1].CanClose || !rows[1].CanForceClose {
		t.Fatalf("unexpected offline row flags: %+v", rows[1])
	}
}

func TestLdkSectionPathHelpers(t *testing.T) {
	if got := templates.LdkSectionOnchain.Path(); got != "/admin/ldk" {
		t.Fatalf("templates.LdkSectionOnchain.Path() = %q", got)
	}
	if got := templates.LdkSectionLightning.Path(); got != "/admin/ldk/lightning" {
		t.Fatalf("templates.LdkSectionLightning.Path() = %q", got)
	}
	if got := templates.LdkSectionPayments.Path(); got != "/admin/ldk/payments" {
		t.Fatalf("templates.LdkSectionPayments.Path() = %q", got)
	}
}

func TestMapLdkChannelRowsZeroTotalBalance(t *testing.T) {
	rows := mapLdkChannelRows([]ldk.LDKChannelSummary{{
		ChannelID:         "chan-zero",
		State:             "pending",
		PeerConnected:     true,
		CounterpartyLabel: "peer-zero",
		CounterpartyPub:   "0333",
		LocalBalanceSats:  0,
		RemoteBalanceSats: 0,
	}})

	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}

	row := rows[0]
	if row.TotalBalanceSats != 0 {
		t.Fatalf("expected zero total balance, got %d", row.TotalBalanceSats)
	}
	if row.LocalBalancePct != 0 || row.RemoteBalancePct != 0 {
		t.Fatalf("expected zero percentages for empty channel, got %+v", row)
	}
	if row.StateLabel != "Pending" || row.CanClose || row.CanForceClose {
		t.Fatalf("unexpected zero-balance row flags: %+v", row)
	}
}

func TestBalancePercents(t *testing.T) {
	tests := []struct {
		name       string
		local      uint64
		remote     uint64
		wantLocal  uint8
		wantRemote uint8
	}{
		{name: "sixty forty", local: 60, remote: 40, wantLocal: 60, wantRemote: 40},
		{name: "zero total", local: 0, remote: 0, wantLocal: 0, wantRemote: 0},
		{name: "one sided", local: 0, remote: 40, wantLocal: 0, wantRemote: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLocal, gotRemote := balancePercents(tt.local, tt.remote)
			if gotLocal != tt.wantLocal || gotRemote != tt.wantRemote {
				t.Fatalf("balancePercents(%d, %d) = (%d, %d), want (%d, %d)", tt.local, tt.remote, gotLocal, gotRemote, tt.wantLocal, tt.wantRemote)
			}
		})
	}
}

func TestMapLdkNetworkSummary(t *testing.T) {
	channels := []ldk.LDKChannelSummary{
		{State: "active", PeerConnected: true},
		{State: "offline", PeerConnected: false},
		{State: "closing", PeerConnected: true},
	}
	peers := []ldk.LDKPeerSummary{
		{NodePub: "peer-a", IsConnected: true},
		{NodePub: "peer-b", IsConnected: false},
	}

	got := mapLdkNetworkSummary(peers, channels)

	if got.TotalPeers != 2 || got.ActivePeers != 1 {
		t.Fatalf("unexpected peer counts: %+v", got)
	}
	if got.TotalChannels != 3 || got.ActiveChannels != 1 {
		t.Fatalf("unexpected channel counts: %+v", got)
	}
}

func TestMapLdkNetworkSummaryZeroValues(t *testing.T) {
	got := mapLdkNetworkSummary(nil, nil)

	if got.TotalPeers != 0 || got.ActivePeers != 0 || got.TotalChannels != 0 || got.ActiveChannels != 0 {
		t.Fatalf("expected zero summary, got %+v", got)
	}
}

func TestMapLdkChannelStateLabel(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{state: "active", want: "Active"},
		{state: "offline", want: "Offline"},
		{state: "closing", want: "Closing"},
		{state: "pending", want: "Pending"},
		{state: "unknown", want: "Pending"},
	}

	for _, tt := range tests {
		if got := mapLdkChannelStateLabel(tt.state); got != tt.want {
			t.Fatalf("mapLdkChannelStateLabel(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCloseEligibilityHelpers(t *testing.T) {
	tests := []struct {
		name           string
		channel        ldk.LDKChannelSummary
		wantCanClose   bool
		wantCanForce   bool
		wantStateLabel string
	}{
		{
			name:           "active",
			channel:        ldk.LDKChannelSummary{State: "active", PeerConnected: true},
			wantCanClose:   true,
			wantCanForce:   false,
			wantStateLabel: "Active",
		},
		{
			name:           "offline",
			channel:        ldk.LDKChannelSummary{State: "offline", PeerConnected: false},
			wantCanClose:   false,
			wantCanForce:   true,
			wantStateLabel: "Offline",
		},
		{
			name:           "pending",
			channel:        ldk.LDKChannelSummary{State: "pending", PeerConnected: true},
			wantCanClose:   false,
			wantCanForce:   false,
			wantStateLabel: "Pending",
		},
		{
			name:           "closing",
			channel:        ldk.LDKChannelSummary{State: "closing", PeerConnected: true},
			wantCanClose:   false,
			wantCanForce:   false,
			wantStateLabel: "Closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canCooperativeClose(tt.channel); got != tt.wantCanClose {
				t.Fatalf("canCooperativeClose(%+v) = %v, want %v", tt.channel, got, tt.wantCanClose)
			}
			if got := canForceClose(tt.channel); got != tt.wantCanForce {
				t.Fatalf("canForceClose(%+v) = %v, want %v", tt.channel, got, tt.wantCanForce)
			}
			if got := mapLdkChannelStateLabel(tt.channel.State); got != tt.wantStateLabel {
				t.Fatalf("mapLdkChannelStateLabel(%q) = %q, want %q", tt.channel.State, got, tt.wantStateLabel)
			}
		})
	}
}

func TestFindLdkChannelByID(t *testing.T) {
	channels := []ldk.LDKChannelSummary{{ChannelID: "chan-1"}, {ChannelID: "chan-2"}}

	if _, err := findLdkChannelByID(channels, ""); err == nil || err.Error() != "channel id is required" {
		t.Fatalf("expected missing id error, got %v", err)
	}
	if _, err := findLdkChannelByID(channels, "missing"); err == nil || err.Error() != "channel not found" {
		t.Fatalf("expected not found error, got %v", err)
	}

	channel, err := findLdkChannelByID(channels, "chan-2")
	if err != nil {
		t.Fatalf("findLdkChannelByID returned error: %v", err)
	}
	if channel.ChannelID != "chan-2" {
		t.Fatalf("expected channel chan-2, got %+v", channel)
	}
}

func TestFindLdkChannelForAction(t *testing.T) {
	channels := []ldk.LDKChannelSummary{{ChannelID: "chan-1", CounterpartyPub: "02aa"}}

	if _, err := findLdkChannelForAction(channels, "", "02aa"); err == nil || err.Error() != "channel id is required" {
		t.Fatalf("expected missing channel id error, got %v", err)
	}
	if _, err := findLdkChannelForAction(channels, "chan-1", ""); err == nil || err.Error() != "counterparty public key is required" {
		t.Fatalf("expected missing counterparty error, got %v", err)
	}
	if _, err := findLdkChannelForAction(channels, "chan-1", "03bb"); err == nil || err.Error() != "channel details are stale, refresh and try again" {
		t.Fatalf("expected stale channel details error, got %v", err)
	}

	channel, err := findLdkChannelForAction(channels, "chan-1", "02aa")
	if err != nil {
		t.Fatalf("findLdkChannelForAction returned error: %v", err)
	}
	if channel.ChannelID != "chan-1" || channel.CounterpartyPub != "02aa" {
		t.Fatalf("unexpected channel returned: %+v", channel)
	}
}

func TestValidateCooperativeClose(t *testing.T) {
	tests := []struct {
		name    string
		channel ldk.LDKChannelSummary
		wantErr string
	}{
		{name: "active", channel: ldk.LDKChannelSummary{State: "active", PeerConnected: true}},
		{name: "offline", channel: ldk.LDKChannelSummary{State: "offline", PeerConnected: false}, wantErr: "channel peer must be connected before starting a cooperative close"},
		{name: "pending", channel: ldk.LDKChannelSummary{State: "pending", PeerConnected: true}, wantErr: "channel is still pending and cannot be closed yet"},
		{name: "closing", channel: ldk.LDKChannelSummary{State: "closing", PeerConnected: true}, wantErr: "channel close is already in progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCooperativeClose(tt.channel)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateCooperativeClose(%+v) returned error: %v", tt.channel, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validateCooperativeClose(%+v) error = %v, want %q", tt.channel, err, tt.wantErr)
			}
		})
	}
}

func TestValidateForceClose(t *testing.T) {
	tests := []struct {
		name    string
		channel ldk.LDKChannelSummary
		wantErr string
	}{
		{name: "offline", channel: ldk.LDKChannelSummary{State: "offline", PeerConnected: false}},
		{name: "active", channel: ldk.LDKChannelSummary{State: "active", PeerConnected: true}, wantErr: "force close is only available while the channel is offline"},
		{name: "pending", channel: ldk.LDKChannelSummary{State: "pending", PeerConnected: true}, wantErr: "channel is still pending and cannot be force closed yet"},
		{name: "closing", channel: ldk.LDKChannelSummary{State: "closing", PeerConnected: true}, wantErr: "channel close is already in progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateForceClose(tt.channel)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateForceClose(%+v) returned error: %v", tt.channel, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validateForceClose(%+v) error = %v, want %q", tt.channel, err, tt.wantErr)
			}
		})
	}
}

func TestParseLdkPeerEndpoint(t *testing.T) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("btcec.NewPrivateKey(): %v", err)
	}
	pubkey := hex.EncodeToString(privKey.PubKey().SerializeCompressed())

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantPubkey  string
		wantAddress string
	}{
		{name: "valid endpoint", input: pubkey + "@172.29.0.2:9735", wantErr: false, wantPubkey: pubkey, wantAddress: "172.29.0.2:9735"},
		{name: "valid endpoint trims spaces", input: "  " + pubkey + "  @  172.29.0.2:9735  ", wantErr: false, wantPubkey: pubkey, wantAddress: "172.29.0.2:9735"},
		{name: "valid non-host address string", input: pubkey + "@remote-peer-address", wantErr: false, wantPubkey: pubkey, wantAddress: "remote-peer-address"},
		{name: "empty input", input: "", wantErr: true},
		{name: "missing separator", input: pubkey + "172.29.0.2:9735", wantErr: true},
		{name: "multiple separators", input: pubkey + "@172.29.0.2@9735", wantErr: true},
		{name: "missing pubkey before separator", input: "@172.29.0.2:9735", wantErr: true},
		{name: "empty address", input: pubkey + "@", wantErr: true},
		{name: "address with invalid whitespace", input: pubkey + "@bad\naddress", wantErr: true},
		{name: "invalid pubkey length", input: "02ab@172.29.0.2:9735", wantErr: true},
		{name: "invalid pubkey bytes", input: "02ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff@172.29.0.2:9735", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedPubkey, parsedAddress, parseErr := parseLdkPeerEndpoint(tt.input)
			if tt.wantErr {
				if parseErr == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}

			if parseErr != nil {
				t.Fatalf("parseLdkPeerEndpoint(%q): %v", tt.input, parseErr)
			}
			if parsedPubkey != tt.wantPubkey {
				t.Fatalf("expected pubkey %q, got %q", tt.wantPubkey, parsedPubkey)
			}
			if parsedAddress != tt.wantAddress {
				t.Fatalf("expected address %q, got %q", tt.wantAddress, parsedAddress)
			}
		})
	}
}

func TestMaxChannelSatsFromOnchain(t *testing.T) {
	tests := []struct {
		onchain uint64
		want    uint64
	}{
		{onchain: 100, want: 95},
		{onchain: 1, want: 0},
		{onchain: 10_001, want: 9_500},
	}

	for _, tt := range tests {
		got := maxChannelSatsFromOnchain(tt.onchain)
		if got != tt.want {
			t.Fatalf("maxChannelSatsFromOnchain(%d) = %d, want %d", tt.onchain, got, tt.want)
		}
	}
}

func TestValidateChannelAmount(t *testing.T) {
	const maxSats = 95

	if err := validateChannelAmount(maxSats, maxSats); err != nil {
		t.Fatalf("validateChannelAmount(max,max) returned error: %v", err)
	}

	if err := validateChannelAmount(maxSats+1, maxSats); err == nil {
		t.Fatal("expected error when amount exceeds max")
	} else if !strings.Contains(err.Error(), "95") {
		t.Fatalf("expected error to include max sats value, got %q", err.Error())
	}

	if err := validateChannelAmount(1, 0); err == nil {
		t.Fatal("expected error when max sats is zero")
	} else if !strings.Contains(strings.ToLower(err.Error()), "too low") {
		t.Fatalf("expected low-balance error, got %q", err.Error())
	}
}

func TestParseLdkOnchainSendAmountRejectsNonNumeric(t *testing.T) {
	_, err := parseLdkOnchainSendAmount("abc")
	if err == nil {
		t.Fatal("expected non-numeric parse error")
	}
}

func TestMapOpenChannelError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "insufficient funds", err: fmt.Errorf("insufficient funds available"), want: "Insufficient on-chain balance to open channel"},
		{name: "address connectivity", err: fmt.Errorf("socket connection failed"), want: "Could not connect to peer address"},
		{name: "pubkey issue", err: fmt.Errorf("invalid pubkey"), want: "Peer public key is invalid"},
		{name: "fallback", err: fmt.Errorf("unexpected internal failure"), want: "Could not open channel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapOpenChannelError(tt.err)
			if got != tt.want {
				t.Fatalf("mapOpenChannelError(%q) = %q, want %q", tt.err.Error(), got, tt.want)
			}
		})
	}
}

func TestMapLdkOnchainSendError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "insufficient funds", err: fmt.Errorf("insufficient spendable balance"), want: "Insufficient available on-chain balance to send funds"},
		{name: "invalid address", err: fmt.Errorf("invalid address checksum"), want: "Destination Bitcoin address is invalid"},
		{name: "fallback", err: fmt.Errorf("broadcast failed"), want: "Could not create or broadcast on-chain payment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapLdkOnchainSendError(tt.err)
			if got != tt.want {
				t.Fatalf("mapLdkOnchainSendError(%q) = %q, want %q", tt.err.Error(), got, tt.want)
			}
		})
	}
}

func TestMapCloseChannelError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		force bool
		want  string
	}{
		{name: "peer connectivity", err: fmt.Errorf("peer is not connected"), want: "Channel peer must be connected before starting a cooperative close"},
		{name: "not found", err: fmt.Errorf("channel not found"), want: "Channel not found"},
		{name: "cooperative fallback", err: fmt.Errorf("close failed"), want: "Unable to start cooperative close"},
		{name: "force fallback", err: fmt.Errorf("force close failed"), force: true, want: "Unable to start force close"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCloseChannelError(tt.err, tt.force)
			if got != tt.want {
				t.Fatalf("mapCloseChannelError(%q, %v) = %q, want %q", tt.err.Error(), tt.force, got, tt.want)
			}
		})
	}
}

func TestWriteLdkMutationSuccessPayload(t *testing.T) {
	rows := []templates.LdkChannelRow{{
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
	err := writeLdkMutationSuccessPayload(
		context.Background(),
		&b,
		rows,
		ldk.LDKBalances{TotalOnchainSats: 100, AvailableOnchainSats: 90, LightningSats: 200},
		nil,
		ldkNetworkSummary{TotalPeers: 2, ActivePeers: 2, TotalChannels: 1, ActiveChannels: 1},
		nil,
		"Channel opening started",
	)
	if err != nil {
		t.Fatalf("writeLdkMutationSuccessPayload(...): %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-channels-fragment\"",
		"id=\"ldk-balances-fragment\" hx-swap-oob=\"outerHTML\"",
		"id=\"ldk-network-summary-fragment\" hx-swap-oob=\"outerHTML\"",
		"Channel opening started",
		"100",
		"90",
		"200",
		"Total On-chain",
		"Available On-chain",
		"Lightning balance",
		"ldk-amount-unit",
		"2 / 2",
		"1 / 1",
		"active / total",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in success payload", check)
		}
	}
	if strings.Contains(out, "id=\"ldk-action-panel\"") {
		t.Fatalf("did not expect action panel replacement in success payload")
	}
}

func TestWriteLdkMutationSuccessPayloadBalanceErrorUsesOOBError(t *testing.T) {
	var b bytes.Buffer
	err := writeLdkMutationSuccessPayload(
		context.Background(),
		&b,
		nil,
		ldk.LDKBalances{},
		fmt.Errorf("boom"),
		ldkNetworkSummary{},
		fmt.Errorf("network boom"),
		"Cooperative close started",
	)
	if err != nil {
		t.Fatalf("writeLdkMutationSuccessPayload(...): %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "hx-swap-oob=\"outerHTML\"") {
		t.Fatalf("expected balances out-of-band swap in error payload")
	}
	if !strings.Contains(out, "Could not refresh LDK balances") {
		t.Fatalf("expected balances refresh error in payload")
	}
	if !strings.Contains(out, "Could not refresh network summary") {
		t.Fatalf("expected network summary refresh error in payload")
	}
}

func TestMutationErrorNotificationDoesNotIncludeOOBBalances(t *testing.T) {
	var b bytes.Buffer
	err := templates.ObbNotification(templates.ErrorNotif("Could not open channel")).Render(context.Background(), &b)
	if err != nil {
		t.Fatalf("ObbNotification(...).Render: %v", err)
	}

	out := b.String()
	if strings.Contains(out, "ldk-balances-fragment") {
		t.Fatalf("did not expect balances fragment in notification-only error payload")
	}
}

func TestWriteLdkOnchainSendSuccessPayload(t *testing.T) {
	var b bytes.Buffer
	err := writeLdkOnchainSendSuccessPayload(
		context.Background(),
		&b,
		ldk.LDKBalances{TotalOnchainSats: 5000, AvailableOnchainSats: 3200},
		"On-chain payment sent",
	)
	if err != nil {
		t.Fatalf("writeLdkOnchainSendSuccessPayload(...): %v", err)
	}

	out := b.String()
	checks := []string{
		"id=\"ldk-balances-fragment\" hx-swap-oob=\"outerHTML\"",
		"id=\"ldk-action-panel\" hx-swap-oob=\"outerHTML\"",
		"Total On-chain",
		"Available On-chain",
		"5.000",
		"3.200",
		"Payment sent",
		"On-chain payment sent",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected %q in on-chain send success payload", check)
		}
	}
	if strings.Contains(out, "ldk-network-summary-fragment") {
		t.Fatalf("did not expect network summary refresh in on-chain send success payload")
	}
	if strings.Contains(out, "ldk-channels-fragment") {
		t.Fatalf("did not expect channel refresh in on-chain send success payload")
	}
}
