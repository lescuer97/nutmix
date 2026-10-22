package admin

import (
	"strings"
	"time"

	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

const (
	ldkPaymentsFilterAll       = "all"
	ldkPaymentsFilterIncoming  = "incoming"
	ldkPaymentsFilterOutgoing  = "outgoing"
	ldkPaymentsUnknownValue    = "Unavailable"
	ldkPaymentsUnknownTime     = "Unknown"
	ldkPaymentsCopyButtonClass = "ldk-payment-copy-btn"
	ldkPaymentsCopyDefaultText = "Copy"
)

type ldkPaymentsError string

func (e ldkPaymentsError) Error() string {
	return string(e)
}

const (
	errInvalidPaymentsFilter ldkPaymentsError = "invalid payment filter"
)

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

func prepareLdkPaymentsPage(payments []ldk_node.PaymentDetails, filter string, cursor string, next string) (templates.LdkPaymentsPage, error) {
	if filter == "" {
		filter = ldkPaymentsFilterAll
	}

	filter, err := parseLdkPaymentsFilter(filter)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	filtered := make([]ldk_node.PaymentDetails, 0, len(payments))
	for _, payment := range payments {
		if includeLdkPaymentDirection(payment.Direction, filter) {
			filtered = append(filtered, payment)
		}
	}

	pageData := templates.LdkPaymentsPage{
		Rows:                  nil,
		ActiveFilter:          filter,
		NextURL:               "",
		FirstURL:              "",
		EmptyMessage:          "",
		ErrorMessage:          "",
		RetryURL:              templates.LdkPaymentsURL(filter, ""),
		CopyButtonClass:       ldkPaymentsCopyButtonClass,
		CopyButtonDefaultText: ldkPaymentsCopyDefaultText,
	}
	if next != "" {
		pageData.NextURL = templates.LdkPaymentsURL(filter, next)
	}
	if cursor != "" {
		pageData.FirstURL = templates.LdkPaymentsURL(filter, "")
	}

	if len(filtered) == 0 {
		pageData.EmptyMessage = ldkPaymentsEmptyMessage()
		return pageData, nil
	}

	pageData.Rows = make([]templates.LdkPaymentRow, 0, len(filtered))
	for _, payment := range filtered {
		pageData.Rows = append(pageData.Rows, mapLdkPaymentRow(payment))
	}

	return pageData, nil
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

func ldkPaymentsEmptyMessage() string {
	return "No matching payments on this page."
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
		Rows:                  nil,
		ActiveFilter:          "",
		NextURL:               "",
		FirstURL:              "",
		EmptyMessage:          "",
		ErrorMessage:          message,
		RetryURL:              templates.LdkPaymentsURL(ldkPaymentsFilterAll, ""),
		CopyButtonClass:       ldkPaymentsCopyButtonClass,
		CopyButtonDefaultText: ldkPaymentsCopyDefaultText,
	}
}

func ldkPaymentsInvalidFilterPage() templates.LdkPaymentsPage {
	return newLdkPaymentsErrorPage("Invalid payment filter")
}

func ldkPaymentsLoadFailurePage() templates.LdkPaymentsPage {
	return newLdkPaymentsErrorPage("Could not load payments")
}

func loadLdkPaymentsPage(payments []ldk_node.PaymentDetails, rawFilter string, cursor string, next string) (templates.LdkPaymentsPage, error) {
	filter, err := parseLdkPaymentsFilter(rawFilter)
	if err != nil {
		return templates.LdkPaymentsPage{}, err
	}
	return prepareLdkPaymentsPage(payments, filter, cursor, next)
}
