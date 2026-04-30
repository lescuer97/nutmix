package templates

import "fmt"

type LdkSection uint8

const (
	LdkSectionOnchain LdkSection = iota
	LdkSectionLightning
	LdkSectionPayments
)

func (s LdkSection) Path() string {
	switch s {
	case LdkSectionLightning:
		return "/admin/ldk/lightning"
	case LdkSectionPayments:
		return "/admin/ldk/payments"
	default:
		return "/admin/ldk"
	}
}

type LdkChannelRow struct {
	ChannelID         string
	CounterpartyLabel string
	CounterpartyPub   string
	LocalBalance      string
	RemoteBalance     string
	StateLabel        string
	LocalBalanceSats  uint64
	RemoteBalanceSats uint64
	TotalBalanceSats  uint64
	CanClose          bool
	CanForceClose     bool
	LocalBalancePct   uint8
	RemoteBalancePct  uint8
}

const (
	LdkPaymentsFilterAll      = "all"
	LdkPaymentsFilterIncoming = "incoming"
	LdkPaymentsFilterOutgoing = "outgoing"
	LdkPaymentsShow25         = "25"
	LdkPaymentsShow100        = "100"
	LdkPaymentsShow150        = "150"
	LdkPaymentsShowAll        = "all"
)

func LdkPaymentsQuery(filter string, show string) string {
	return fmt.Sprintf("?type=%s&show=%s", filter, show)
}

//nolint:govet // Templ view model keeps related UI fields grouped for readability.
type LdkPaymentsPage struct {
	ShowOptions           []LdkPaymentsShowOptionData
	Rows                  []LdkPaymentRow
	ActiveFilter          string
	SelectedShow          string
	EmptyMessage          string
	ErrorMessage          string
	RetryQuery            string
	CopyButtonClass       string
	CopyButtonDefaultText string
	TotalItems            int
	ShowingFrom           int
	ShowingTo             int
}

type LdkPaymentsShowOptionData struct {
	Label    string
	Value    string
	Query    string
	Selected bool
}

type LdkPaymentRow struct {
	DirectionLabel         string
	DirectionKey           string
	KindBadgeLabel         string
	Amount                 string
	StatusLabel            string
	IdentifierLabel        string
	IdentifierValue        string
	ShortIdentifierValue   string
	FormattedLastUpdatedAt string
	CopyPayload            string
	CanCopy                bool
}
