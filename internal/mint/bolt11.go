package mint

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lightningnetwork/lnd/invoices"
)

func CheckMintRequest(pool *pgxpool.Pool, mint *Mint, quoteId string) (cashu.PostMintQuoteBolt11Response, error) {
	quote, err := database.GetMintQuoteById(pool, quoteId)

	if err != nil {
		return quote.PostMintQuoteBolt11Response(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}
	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.PostMintQuoteBolt11Response(), nil
	}

	state, _, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMintPayStatus)

	if err != nil {
		return quote.PostMintQuoteBolt11Response(), fmt.Errorf("mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid. %w", err)
	}

	quote.State = state

	if state == cashu.PAID {
		quote.RequestPaid = true
	}
	return quote.PostMintQuoteBolt11Response(), nil

}

func CheckMeltRequest(pool *pgxpool.Pool, mint *Mint, quoteId string) (cashu.PostMeltQuoteBolt11Response, error) {
	quote, err := database.GetMeltQuoteById(pool, quoteId)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}

	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.GetPostMeltQuoteResponse(), nil
	}

	state, preimage, err := mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMeltPayStatus)
	if err != nil {
		if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
			return quote.GetPostMeltQuoteResponse(), nil
		}

		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid, quote.Quote, database.ModifyQuoteMeltPayStatus) %w", err)
	}
	quote.PaymentPreimage = preimage
	quote.State = state
	if state == cashu.PAID {
		quote.RequestPaid = true
	}

	err = database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote) %w", err)
	}

	return quote.GetPostMeltQuoteResponse(), nil

}
