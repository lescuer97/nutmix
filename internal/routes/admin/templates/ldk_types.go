package templates

import (
	"net/url"
)

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
)

func LdkPaymentsURL(filter string, cursor string) string {
	values := url.Values{}
	values.Set("type", filter)
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	return "/admin/ldk/payments?" + values.Encode()
}

//nolint:govet // Templ view model keeps related UI fields grouped for readability.
type LdkPaymentsPage struct {
	Rows                  []LdkPaymentRow
	ActiveFilter          string
	NextURL               string
	FirstURL              string
	EmptyMessage          string
	ErrorMessage          string
	RetryURL              string
	CopyButtonClass       string
	CopyButtonDefaultText string
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
