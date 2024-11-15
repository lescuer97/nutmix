package mint

import (
	"errors"
	"fmt"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lightningnetwork/lnd/invoices"
	"strings"
)

func CheckMintRequest(mint *Mint, quoteId string) (cashu.PostMintQuoteBolt11Response, error) {
	quote, err := mint.MintDB.GetMintRequestById(quoteId)
	if err != nil {
		return quote.PostMintQuoteBolt11Response(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}

	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.PostMintQuoteBolt11Response(), nil
	}

	state, _, err := mint.VerifyLightingPaymentHappened(quote.RequestPaid, quote.Quote, mint.MintDB.ChangeMintRequestState)

	if err != nil {
		return quote.PostMintQuoteBolt11Response(), fmt.Errorf("mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid. %w", err)
	}

	quote.State = state

	if state == cashu.PAID {
		quote.RequestPaid = true
	}
	return quote.PostMintQuoteBolt11Response(), nil

}

func CheckMeltRequest(mint *Mint, quoteId string) (cashu.PostMeltQuoteBolt11Response, error) {

	quote, err := mint.MintDB.GetMeltRequestById(quoteId)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}

	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.GetPostMeltQuoteResponse(), nil
	}

	state, preimage, err := mint.VerifyLightingPaymentHappened(quote.RequestPaid, quote.Quote, mint.MintDB.ChangeMeltRequestState)
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

	err = mint.MintDB.AddPreimageMeltRequest(quote.Quote, preimage)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote) %w", err)
	}

	return quote.GetPostMeltQuoteResponse(), nil

}
