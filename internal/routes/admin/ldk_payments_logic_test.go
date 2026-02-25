package admin

import (
	"fmt"
	"testing"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
)

func TestParseLdkPaymentsFilter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{name: "missing defaults all", raw: "", want: ldkPaymentsFilterAll},
		{name: "incoming", raw: "incoming", want: ldkPaymentsFilterIncoming},
		{name: "outgoing uppercase", raw: " OUTGOING ", want: ldkPaymentsFilterOutgoing},
		{name: "invalid", raw: "sideways", wantErr: errInvalidPaymentsFilter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLdkPaymentsFilter(tt.raw)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("parseLdkPaymentsFilter(%q) error = %v, want %v", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLdkPaymentsFilter(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("parseLdkPaymentsFilter(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseLdkPaymentsShow(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantMax int
		wantErr error
	}{
		{name: "missing defaults 25", raw: "", want: ldkPaymentsShow25, wantMax: 25},
		{name: "show 100", raw: "100", want: ldkPaymentsShow100, wantMax: 100},
		{name: "show all", raw: "all", want: ldkPaymentsShowAll, wantMax: -1},
		{name: "invalid", raw: "80", wantErr: errInvalidPaymentsShow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotMax, err := parseLdkPaymentsShow(tt.raw)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("parseLdkPaymentsShow(%q) error = %v, want %v", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLdkPaymentsShow(%q) error = %v", tt.raw, err)
			}
			if got != tt.want || gotMax != tt.wantMax {
				t.Fatalf("parseLdkPaymentsShow(%q) = (%q, %d), want (%q, %d)", tt.raw, got, gotMax, tt.want, tt.wantMax)
			}
		})
	}
}

func TestPrepareLdkPaymentsPageShowAndFiltering(t *testing.T) {
	payments := make([]ldk_node.PaymentDetails, 0, 31)
	for i := 0; i < 28; i++ {
		direction := ldk_node.PaymentDirectionInbound
		id := paymentID(i)
		if i%2 == 1 {
			direction = ldk_node.PaymentDirectionOutbound
		}
		payments = append(payments, ldk_node.PaymentDetails{
			Id:                    id,
			Kind:                  ldk_node.PaymentKindBolt11{Hash: "hash-" + id},
			AmountMsat:            uint64Ptr(1000),
			Direction:             direction,
			Status:                ldk_node.PaymentStatusPending,
			LatestUpdateTimestamp: uint64(1000 + i),
		})
	}
	payments = append(payments,
		ldk_node.PaymentDetails{Id: "unknown-new", Direction: ldk_node.PaymentDirection(99), LatestUpdateTimestamp: 5000},
		ldk_node.PaymentDetails{Id: "unknown-old", Direction: ldk_node.PaymentDirection(99), LatestUpdateTimestamp: 10},
		ldk_node.PaymentDetails{Id: "latest-out", Direction: ldk_node.PaymentDirectionOutbound, LatestUpdateTimestamp: 6000, Kind: ldk_node.PaymentKindBolt11{Hash: "latest-out"}, AmountMsat: uint64Ptr(1000)},
	)

	allPage, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterAll, ldkPaymentsShow25)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(all, 25) error = %v", err)
	}
	if allPage.TotalItems != 31 || allPage.SelectedShow != ldkPaymentsShow25 {
		t.Fatalf("unexpected all page totals: %+v", allPage)
	}
	if allPage.ShowingFrom != 1 || allPage.ShowingTo != 25 {
		t.Fatalf("unexpected all page showing range: %+v", allPage)
	}
	if len(allPage.ShowOptions) != 4 || !allPage.ShowOptions[0].Selected || allPage.ShowOptions[3].Selected {
		t.Fatalf("unexpected show options: %+v", allPage.ShowOptions)
	}
	if allPage.Rows[0].IdentifierValue != "latest-out" {
		t.Fatalf("expected newest payment first, got %+v", allPage.Rows[0])
	}
	if allPage.Rows[1].DirectionLabel != "Unknown Payment" {
		t.Fatalf("expected unknown direction visible in all filter, got %+v", allPage.Rows[1])
	}

	showAllPage, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterAll, ldkPaymentsShowAll)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(all, all) error = %v", err)
	}
	if showAllPage.ShowingFrom != 1 || showAllPage.ShowingTo != 31 || len(showAllPage.Rows) != 31 {
		t.Fatalf("unexpected all-show range: %+v", showAllPage)
	}

	incomingPage, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterIncoming, ldkPaymentsShow25)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(incoming, 25) error = %v", err)
	}
	for _, row := range incomingPage.Rows {
		if row.DirectionKey != "inbound" {
			t.Fatalf("incoming page included non-inbound row: %+v", row)
		}
	}

	outgoingPage, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterOutgoing, ldkPaymentsShow25)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(outgoing, 25) error = %v", err)
	}
	for _, row := range outgoingPage.Rows {
		if row.DirectionKey != "outbound" {
			t.Fatalf("outgoing page included non-outbound row: %+v", row)
		}
	}
}

func TestPrepareLdkPaymentsPageValidation(t *testing.T) {
	payments := []ldk_node.PaymentDetails{{Id: "one", Direction: ldk_node.PaymentDirectionInbound, LatestUpdateTimestamp: 1}}

	if _, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterAll, "80"); err != errInvalidPaymentsShow {
		t.Fatalf("expected invalid show error, got %v", err)
	}
	if _, err := prepareLdkPaymentsPage(payments, "sideways", ldkPaymentsShow25); err != errInvalidPaymentsFilter {
		t.Fatalf("expected invalid filter error, got %v", err)
	}

	emptyPage, err := prepareLdkPaymentsPage(nil, ldkPaymentsFilterOutgoing, ldkPaymentsShow150)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(empty, 150) error = %v", err)
	}
	if emptyPage.ShowingFrom != 0 || emptyPage.ShowingTo != 0 || emptyPage.EmptyMessage != "No outgoing payments found." {
		t.Fatalf("unexpected empty page: %+v", emptyPage)
	}
}

func TestPrepareLdkPaymentsPageTieBreakers(t *testing.T) {
	payments := []ldk_node.PaymentDetails{
		{Id: "", LatestUpdateTimestamp: 100, Direction: ldk_node.PaymentDirectionInbound},
		{Id: "", LatestUpdateTimestamp: 100, Direction: ldk_node.PaymentDirectionInbound},
		{Id: "aaa", LatestUpdateTimestamp: 100, Direction: ldk_node.PaymentDirectionInbound},
		{Id: "bbb", LatestUpdateTimestamp: 100, Direction: ldk_node.PaymentDirectionInbound},
	}

	page, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterAll, ldkPaymentsShow25)
	if err != nil {
		t.Fatalf("prepareLdkPaymentsPage(...) error = %v", err)
	}

	got := []string{
		page.Rows[0].IdentifierValue,
		page.Rows[1].IdentifierValue,
		page.Rows[2].IdentifierValue,
		page.Rows[3].IdentifierValue,
	}
	want := []string{ldkPaymentsUnknownValue, ldkPaymentsUnknownValue, "aaa", "bbb"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d identifier = %q, want %q (all rows=%+v)", i, got[i], want[i], page.Rows)
		}
	}
}

func TestMapLdkPaymentRow(t *testing.T) {
	timestamp := uint64(1711111111)
	amountMsat := uint64(123456)

	tests := []struct {
		name                string
		payment             ldk_node.PaymentDetails
		wantDirectionLabel  string
		wantDirectionKey    string
		wantKind            string
		wantStatus          string
		wantIdentifierLabel string
		wantIdentifierValue string
		wantAmount          string
		wantCanCopy         bool
	}{
		{
			name:                "onchain",
			payment:             ldk_node.PaymentDetails{Id: "fallback", Kind: ldk_node.PaymentKindOnchain{Txid: "tx-123"}, Direction: ldk_node.PaymentDirectionInbound, Status: ldk_node.PaymentStatusSucceeded, AmountMsat: &amountMsat, LatestUpdateTimestamp: timestamp},
			wantDirectionLabel:  "Inbound Payment",
			wantDirectionKey:    "inbound",
			wantKind:            "ON-CHAIN",
			wantStatus:          "Succeeded",
			wantIdentifierLabel: "TRANSACTION ID",
			wantIdentifierValue: "tx-123",
			wantAmount:          "123 sats",
			wantCanCopy:         true,
		},
		{
			name:                "bolt12 fallback to payment id",
			payment:             ldk_node.PaymentDetails{Id: "payment-id-123456789", Kind: ldk_node.PaymentKindBolt12Offer{}, Direction: ldk_node.PaymentDirectionOutbound, Status: ldk_node.PaymentStatusPending, LatestUpdateTimestamp: timestamp},
			wantDirectionLabel:  "Outbound Payment",
			wantDirectionKey:    "outbound",
			wantKind:            "LIGHTNING",
			wantStatus:          "Pending",
			wantIdentifierLabel: "PAYMENT ID",
			wantIdentifierValue: "payment-id-123456789",
			wantAmount:          ldkPaymentsUnknownValue,
			wantCanCopy:         true,
		},
		{
			name:                "unknown kind and direction",
			payment:             ldk_node.PaymentDetails{Direction: ldk_node.PaymentDirection(99), Status: ldk_node.PaymentStatus(99), LatestUpdateTimestamp: 0},
			wantDirectionLabel:  "Unknown Payment",
			wantDirectionKey:    "unknown",
			wantKind:            "UNKNOWN",
			wantStatus:          "Unknown",
			wantIdentifierLabel: "PAYMENT ID",
			wantIdentifierValue: ldkPaymentsUnknownValue,
			wantAmount:          ldkPaymentsUnknownValue,
			wantCanCopy:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := mapLdkPaymentRow(tt.payment)
			if row.DirectionLabel != tt.wantDirectionLabel || row.DirectionKey != tt.wantDirectionKey {
				t.Fatalf("unexpected direction mapping: %+v", row)
			}
			if row.KindBadgeLabel != tt.wantKind || row.StatusLabel != tt.wantStatus {
				t.Fatalf("unexpected kind/status mapping: %+v", row)
			}
			if row.IdentifierLabel != tt.wantIdentifierLabel || row.IdentifierValue != tt.wantIdentifierValue {
				t.Fatalf("unexpected identifier mapping: %+v", row)
			}
			if row.Amount != tt.wantAmount || row.CanCopy != tt.wantCanCopy {
				t.Fatalf("unexpected amount/copy mapping: %+v", row)
			}
		})
	}
}

func TestMapLdkPaymentRowFormatting(t *testing.T) {
	amount := uint64(999)
	row := mapLdkPaymentRow(ldk_node.PaymentDetails{
		Id:                    "12345678901234567890",
		Kind:                  ldk_node.PaymentKindBolt11{Hash: "hash-12345678901234567890"},
		AmountMsat:            &amount,
		Direction:             ldk_node.PaymentDirectionInbound,
		Status:                ldk_node.PaymentStatusFailed,
		LatestUpdateTimestamp: 1711111111,
	})

	if row.Amount != "0 sats" {
		t.Fatalf("expected sub-sat amount to floor to 0 sats, got %q", row.Amount)
	}
	if row.ShortIdentifierValue != "hash-1234567..." {
		t.Fatalf("unexpected shortened identifier: %q", row.ShortIdentifierValue)
	}
	if row.FormattedLastUpdatedAt != "2024-03-22 12:38:31 UTC" {
		t.Fatalf("unexpected formatted timestamp: %q", row.FormattedLastUpdatedAt)
	}
}

func TestLoadLdkPaymentsPageAndErrorMapping(t *testing.T) {
	payments := []ldk_node.PaymentDetails{{Id: "one", Direction: ldk_node.PaymentDirectionInbound, LatestUpdateTimestamp: 1}}

	page, err := loadLdkPaymentsPage(payments, "", "")
	if err != nil {
		t.Fatalf("loadLdkPaymentsPage(...) error = %v", err)
	}
	if page.ActiveFilter != ldkPaymentsFilterAll || page.SelectedShow != ldkPaymentsShow25 {
		t.Fatalf("unexpected defaulted page: %+v", page)
	}

	if got := ldkPaymentsPageForError(errInvalidPaymentsFilter); got.ErrorMessage != "Invalid payment filter" {
		t.Fatalf("unexpected invalid filter page: %+v", got)
	}
	if got := ldkPaymentsPageForError(errInvalidPaymentsShow); got.ErrorMessage != "Invalid payments show value" {
		t.Fatalf("unexpected invalid show page: %+v", got)
	}
	if got := ldkPaymentsPageForError(nil); got.ErrorMessage != "Could not load payments" {
		t.Fatalf("unexpected load failure page: %+v", got)
	}
}

func uint64Ptr(v uint64) *uint64 { return &v }

func paymentID(i int) string { return fmt.Sprintf("payment-%02d", i) }
