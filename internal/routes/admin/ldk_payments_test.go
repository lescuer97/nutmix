package admin

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
)

func TestLdkPaymentsPageRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := ldkPaymentsPageLoader
	t.Cleanup(func() { ldkPaymentsPageLoader = old })
	calls := 0
	var gotType ldk.PaymentType
	var gotCursor string
	ldkPaymentsPageLoader = func(_ *ldk.LDK, typ ldk.PaymentType, cursor string) ([]ldk_node.PaymentDetails, string, error) {
		calls++
		gotType = typ
		gotCursor = cursor
		return []ldk_node.PaymentDetails{{Id: "p", Kind: ldk_node.PaymentKindBolt11{}, Direction: ldk_node.PaymentDirectionInbound}}, "next /+?&", nil
	}
	m := &mint.Mint{LightningBackend: &ldk.LDK{}}
	request := httptest.NewRequest("GET", "/admin/ldk/payments?type=incoming&cursor=a%2Fb%2B%3F%26", nil)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = request
	LdkPaymentsPage(m)(c)
	if calls != 1 || gotType != ldk.Incoming || gotCursor != "a/b+?&" {
		t.Fatalf("calls=%d type=%v cursor=%q", calls, gotType, gotCursor)
	}
	if !strings.Contains(response.Body.String(), "href=\"/admin/ldk/payments?cursor=next+%2F%2B%3F%26&amp;type=incoming\"") {
		t.Fatalf("missing encoded Next link: %s", response.Body.String())
	}
}

func TestLdkPaymentsInvalidFilterSkipsLoader(t *testing.T) {
	old := ldkPaymentsPageLoader
	t.Cleanup(func() { ldkPaymentsPageLoader = old })
	calls := 0
	ldkPaymentsPageLoader = func(*ldk.LDK, ldk.PaymentType, string) ([]ldk_node.PaymentDetails, string, error) {
		calls++
		return nil, "", nil
	}
	page, err := loadLdkPaymentsRequest(&mint.Mint{LightningBackend: &ldk.LDK{}}, "bad", "cursor")
	if err != errInvalidPaymentsFilter || calls != 0 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
	page = paymentsErrorPage("bad", "cursor", err)
	if page.ErrorMessage != "Invalid payment filter" {
		t.Fatalf("page=%+v", page)
	}
}

func TestLdkPaymentsInvalidCursorRecovery(t *testing.T) {
	page := paymentsErrorPage("outgoing", "opaque", fmt.Errorf("list payments: %w", ldk_node.ErrNodeErrorInvalidPageToken))
	if page.ErrorMessage != "Invalid payment cursor" {
		t.Fatalf("error message = %q, want invalid cursor recovery", page.ErrorMessage)
	}
	if page.FirstURL != "/admin/ldk/payments?type=outgoing" || page.RetryURL != "/admin/ldk/payments?cursor=opaque&type=outgoing" {
		t.Fatalf("page=%+v", page)
	}
}
