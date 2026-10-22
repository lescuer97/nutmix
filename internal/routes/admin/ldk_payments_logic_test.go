package admin

import (
	"testing"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

func TestLdkPaymentsPaginationModel(t *testing.T) {
	const cursor = "opaque /+?&= token"
	if got := templates.LdkPaymentsURL("incoming", cursor); got != "/admin/ldk/payments?cursor=opaque+%2F%2B%3F%26%3D+token&type=incoming" {
		t.Fatalf("URL = %q", got)
	}
	payments := []ldk_node.PaymentDetails{
		{Id: "first", Direction: ldk_node.PaymentDirectionInbound, Kind: ldk_node.PaymentKindBolt11{}, LatestUpdateTimestamp: 1},
		{Id: "second", Direction: ldk_node.PaymentDirectionOutbound, Kind: ldk_node.PaymentKindBolt11{}, LatestUpdateTimestamp: 99},
	}
	page, err := prepareLdkPaymentsPage(payments, ldkPaymentsFilterAll, "", "next token")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 2 || page.Rows[0].IdentifierValue != "first" || page.Rows[1].IdentifierValue != "second" {
		t.Fatalf("native order lost: %+v", page.Rows)
	}
	if page.NextURL != templates.LdkPaymentsURL("all", "next token") {
		t.Fatalf("next URL %q", page.NextURL)
	}
	filtered, err := prepareLdkPaymentsPage(nil, ldkPaymentsFilterOutgoing, cursor, "next")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.EmptyMessage != "No matching payments on this page." || filtered.NextURL == "" || filtered.FirstURL != templates.LdkPaymentsURL("outgoing", "") {
		t.Fatalf("empty pagination model: %+v", filtered)
	}
	if _, err := prepareLdkPaymentsPage(nil, "invalid", "", ""); err != errInvalidPaymentsFilter {
		t.Fatalf("invalid filter err=%v", err)
	}
}

func TestParseLdkPaymentsFilter(t *testing.T) {
	for raw, want := range map[string]string{"": "all", "incoming": "incoming", " OUTGOING ": "outgoing"} {
		got, err := parseLdkPaymentsFilter(raw)
		if err != nil || got != want {
			t.Fatalf("parse(%q)=(%q,%v), want %q", raw, got, err, want)
		}
	}
	if _, err := parseLdkPaymentsFilter("invalid"); err != errInvalidPaymentsFilter {
		t.Fatalf("error=%v", err)
	}
}
