package admin

import (
	"fmt"
	"sort"
	"strings"
	"time"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

const (
	ldkPaymentsFilterAll         = "all"
	ldkPaymentsFilterIncoming    = "incoming"
	ldkPaymentsFilterOutgoing    = "outgoing"
	ldkPaymentsShow25            = "25"
	ldkPaymentsShow100           = "100"
	ldkPaymentsShow150           = "150"
	ldkPaymentsShowAll           = "all"
	ldkPaymentsUnknownValue      = "Unavailable"
	ldkPaymentsUnknownTime       = "Unknown"
	ldkPaymentsCopyButtonClass   = "ldk-payment-copy-btn"
	ldkPaymentsCopyDefaultText   = "Copy"
	ldkPaymentsDefaultRetryQuery = "?type=all&show=25"
)

type ldkPaymentsError string

func (e ldkPaymentsError) Error() string {
	return string(e)
}

const (
	errInvalidPaymentsFilter ldkPaymentsError = "invalid payment filter"
	errInvalidPaymentsShow   ldkPaymentsError = "invalid payments show"
)

type indexedPayment struct {
	payment ldk_node.PaymentDetails
	index   int
}

func parseLdkPaymentsFilter(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ldkPaymentsFilterAll, nil
	}

	switch value {
	case ldkPaymentsFilterAll, ldkPaymentsFilterIncoming, ldkPaymentsFilterOutgoing:
		return value, nil
	default:
		return "", errInvalidPaymentsFilter
	}
}

func parseLdkPaymentsShow(raw string) (string, int, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ldkPaymentsShow25, 25, nil
	}

	switch value {
	case ldkPaymentsShow25:
		return value, 25, nil
	case ldkPaymentsShow100:
		return value, 100, nil
	case ldkPaymentsShow150:
		return value, 150, nil
	case ldkPaymentsShowAll:
		return value, -1, nil
	default:
		return "", 0, errInvalidPaymentsShow
	}
}

func buildLdkPaymentsQuery(filter string, show string) string {
	return fmt.Sprintf("?type=%s&show=%s", filter, show)
}

func prepareLdkPaymentsPage(payments []ldk_node.PaymentDetails, filter string, show string) (templates.LdkPaymentsPage, error) {
	if filter == "" {
		filter = ldkPaymentsFilterAll
	}

	filter, err := parseLdkPaymentsFilter(filter)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	selectedShow, limit, err := parseLdkPaymentsShow(show)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}

	prepared := make([]indexedPayment, 0, len(payments))
	for i, payment := range payments {
		prepared = append(prepared, indexedPayment{payment: payment, index: i})
	}

	sort.SliceStable(prepared, func(i, j int) bool {
		left := prepared[i]
		right := prepared[j]
		if left.payment.LatestUpdateTimestamp != right.payment.LatestUpdateTimestamp {
			return left.payment.LatestUpdateTimestamp > right.payment.LatestUpdateTimestamp
		}
		if left.payment.Id != right.payment.Id {
			return left.payment.Id < right.payment.Id
		}
		return left.index < right.index
	})

	filtered := make([]indexedPayment, 0, len(prepared))
	for _, payment := range prepared {
		if includeLdkPaymentDirection(payment.payment.Direction, filter) {
			filtered = append(filtered, payment)
		}
	}

	pageData := templates.LdkPaymentsPage{
		ShowOptions:           buildLdkPaymentsShowOptions(filter, selectedShow),
		Rows:                  nil,
		ActiveFilter:          filter,
		SelectedShow:          selectedShow,
		EmptyMessage:          "",
		ErrorMessage:          "",
		RetryQuery:            ldkPaymentsDefaultRetryQuery,
		CopyButtonClass:       ldkPaymentsCopyButtonClass,
		CopyButtonDefaultText: ldkPaymentsCopyDefaultText,
		TotalItems:            len(filtered),
		ShowingFrom:           0,
		ShowingTo:             0,
	}

	if len(filtered) == 0 {
		pageData.EmptyMessage = ldkPaymentsEmptyMessage(filter)
		return pageData, nil
	}

	end := len(filtered)
	if limit > 0 && limit < end {
		end = limit
	}

	pageData.ShowingFrom = 1
	pageData.ShowingTo = end

	pageData.Rows = make([]templates.LdkPaymentRow, 0, end)
	for _, payment := range filtered[:end] {
		pageData.Rows = append(pageData.Rows, mapLdkPaymentRow(payment.payment))
	}

	return pageData, nil
}

func buildLdkPaymentsShowOptions(filter string, selectedShow string) []templates.LdkPaymentsShowOptionData {
	options := []templates.LdkPaymentsShowOptionData{
		{Label: "25", Value: ldkPaymentsShow25, Query: "", Selected: false},
		{Label: "100", Value: ldkPaymentsShow100, Query: "", Selected: false},
		{Label: "150", Value: ldkPaymentsShow150, Query: "", Selected: false},
		{Label: "ALL", Value: ldkPaymentsShowAll, Query: "", Selected: false},
	}
	for i := range options {
		options[i].Query = buildLdkPaymentsQuery(filter, options[i].Value)
		options[i].Selected = options[i].Value == selectedShow
	}
	return options
}

func includeLdkPaymentDirection(direction ldk_node.PaymentDirection, filter string) bool {
	switch filter {
	case ldkPaymentsFilterAll:
		return true
	case ldkPaymentsFilterIncoming:
		return direction == ldk_node.PaymentDirectionInbound
	case ldkPaymentsFilterOutgoing:
		return direction == ldk_node.PaymentDirectionOutbound
	default:
		return false
	}
}

func ldkPaymentsEmptyMessage(filter string) string {
	switch filter {
	case ldkPaymentsFilterIncoming:
		return "No incoming payments found."
	case ldkPaymentsFilterOutgoing:
		return "No outgoing payments found."
	default:
		return "No payments found."
	}
}

func mapLdkPaymentRow(payment ldk_node.PaymentDetails) templates.LdkPaymentRow {
	directionLabel, directionKey := mapLdkPaymentDirection(payment.Direction)
	kindBadgeLabel, identifierLabel, identifierValue := mapLdkPaymentIdentifier(payment)
	shortIdentifierValue := shortenLdkPaymentIdentifier(identifierValue)
	canCopy := identifierValue != ldkPaymentsUnknownValue

	return templates.LdkPaymentRow{
		DirectionLabel:         directionLabel,
		DirectionKey:           directionKey,
		KindBadgeLabel:         kindBadgeLabel,
		Amount:                 mapLdkPaymentAmount(payment.AmountMsat),
		StatusLabel:            mapLdkPaymentStatus(payment.Status),
		IdentifierLabel:        identifierLabel,
		IdentifierValue:        identifierValue,
		ShortIdentifierValue:   shortIdentifierValue,
		FormattedLastUpdatedAt: formatLdkPaymentTimestamp(payment.LatestUpdateTimestamp),
		CopyPayload:            identifierValue,
		CanCopy:                canCopy,
	}
}

func mapLdkPaymentDirection(direction ldk_node.PaymentDirection) (string, string) {
	switch direction {
	case ldk_node.PaymentDirectionInbound:
		return "Inbound Payment", "inbound"
	case ldk_node.PaymentDirectionOutbound:
		return "Outbound Payment", "outbound"
	default:
		return "Unknown Payment", "unknown"
	}
}

func mapLdkPaymentIdentifier(payment ldk_node.PaymentDetails) (string, string, string) {
	switch kind := payment.Kind.(type) {
	case ldk_node.PaymentKindOnchain:
		return "ON-CHAIN", "TRANSACTION ID", preferredLdkPaymentIdentifier(kind.Txid, payment.Id)
	case *ldk_node.PaymentKindOnchain:
		if kind == nil {
			break
		}
		return "ON-CHAIN", "TRANSACTION ID", preferredLdkPaymentIdentifier(kind.Txid, payment.Id)
	case ldk_node.PaymentKindBolt11:
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	case *ldk_node.PaymentKindBolt11:
		if kind == nil {
			break
		}
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	case ldk_node.PaymentKindBolt11Jit:
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	case *ldk_node.PaymentKindBolt11Jit:
		if kind == nil {
			break
		}
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	case ldk_node.PaymentKindBolt12Offer:
		if kind.Hash != nil && *kind.Hash != "" {
			return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(*kind.Hash, payment.Id)
		}
		return "LIGHTNING", "PAYMENT ID", preferredLdkPaymentIdentifier("", payment.Id)
	case *ldk_node.PaymentKindBolt12Offer:
		if kind == nil {
			break
		}
		if kind.Hash != nil && *kind.Hash != "" {
			return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(*kind.Hash, payment.Id)
		}
		return "LIGHTNING", "PAYMENT ID", preferredLdkPaymentIdentifier("", payment.Id)
	case ldk_node.PaymentKindBolt12Refund:
		if kind.Hash != nil && *kind.Hash != "" {
			return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(*kind.Hash, payment.Id)
		}
		return "LIGHTNING", "PAYMENT ID", preferredLdkPaymentIdentifier("", payment.Id)
	case *ldk_node.PaymentKindBolt12Refund:
		if kind == nil {
			break
		}
		if kind.Hash != nil && *kind.Hash != "" {
			return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(*kind.Hash, payment.Id)
		}
		return "LIGHTNING", "PAYMENT ID", preferredLdkPaymentIdentifier("", payment.Id)
	case ldk_node.PaymentKindSpontaneous:
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	case *ldk_node.PaymentKindSpontaneous:
		if kind == nil {
			break
		}
		return "LIGHTNING", "PAYMENT HASH", preferredLdkPaymentIdentifier(kind.Hash, payment.Id)
	}

	return "UNKNOWN", "PAYMENT ID", preferredLdkPaymentIdentifier("", payment.Id)
}

func preferredLdkPaymentIdentifier(preferred string, fallback string) string {
	if strings.TrimSpace(preferred) != "" {
		return preferred
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return ldkPaymentsUnknownValue
}

func mapLdkPaymentAmount(amountMsat *uint64) string {
	if amountMsat == nil {
		return ldkPaymentsUnknownValue
	}
	return templates.FormatNumber(*amountMsat/1000) + " sats"
}

func mapLdkPaymentStatus(status ldk_node.PaymentStatus) string {
	switch status {
	case ldk_node.PaymentStatusSucceeded:
		return "Succeeded"
	case ldk_node.PaymentStatusPending:
		return "Pending"
	case ldk_node.PaymentStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

func shortenLdkPaymentIdentifier(identifier string) string {
	if len(identifier) <= 16 {
		return identifier
	}
	return identifier[:12] + "..."
}

func formatLdkPaymentTimestamp(timestamp uint64) string {
	if timestamp == 0 {
		return ldkPaymentsUnknownTime
	}
	return time.Unix(int64(timestamp), 0).UTC().Format("2006-01-02 15:04:05 UTC")
}

func newLdkPaymentsErrorPage(message string) templates.LdkPaymentsPage {
	return templates.LdkPaymentsPage{
		ShowOptions:           nil,
		Rows:                  nil,
		ActiveFilter:          "",
		SelectedShow:          "",
		EmptyMessage:          "",
		ErrorMessage:          message,
		RetryQuery:            ldkPaymentsDefaultRetryQuery,
		CopyButtonClass:       ldkPaymentsCopyButtonClass,
		CopyButtonDefaultText: ldkPaymentsCopyDefaultText,
		TotalItems:            0,
		ShowingFrom:           0,
		ShowingTo:             0,
	}
}

func ldkPaymentsInvalidFilterPage() templates.LdkPaymentsPage {
	return newLdkPaymentsErrorPage("Invalid payment filter")
}

func ldkPaymentsInvalidShowPage() templates.LdkPaymentsPage {
	return newLdkPaymentsErrorPage("Invalid payments show value")
}

func ldkPaymentsLoadFailurePage() templates.LdkPaymentsPage {
	return newLdkPaymentsErrorPage("Could not load payments")
}

func loadLdkPaymentsPage(allPayments []ldk_node.PaymentDetails, rawFilter string, rawPage string) (templates.LdkPaymentsPage, error) {
	filter, err := parseLdkPaymentsFilter(rawFilter)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	selectedShow, _, err := parseLdkPaymentsShow(rawPage)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	return prepareLdkPaymentsPage(allPayments, filter, selectedShow)
}

func ldkPaymentsPageForError(err error) templates.LdkPaymentsPage {
	switch err {
	case errInvalidPaymentsFilter:
		return ldkPaymentsInvalidFilterPage()
	case errInvalidPaymentsShow:
		return ldkPaymentsInvalidShowPage()
	default:
		return ldkPaymentsLoadFailurePage()
	}
}
