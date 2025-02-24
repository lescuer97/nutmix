package mint

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/invoices"
)

func CheckMintRequest(mint *Mint, quote cashu.MintRequestDB) (cashu.MintRequestDB, error) {

	status, _, err := mint.LightningBackend.CheckReceived(quote.Quote)
	if err != nil {
		return quote, fmt.Errorf("mint.VerifyLightingPaymentHappened(pool, quote.RequestPaid. %w", err)
	}
	switch {
	case status == lightning.SETTLED:
		quote.State = cashu.PAID
		quote.RequestPaid = true

	case status == lightning.PENDING:
		quote.State = cashu.PENDING
	case status == lightning.FAILED:
		quote.State = cashu.UNPAID

	}
	return quote, nil

}

func CheckMeltRequest(mint *Mint, quoteId string) (cashu.PostMeltQuoteBolt11Response, error) {

	tx, err := mint.MintDB.GetTx(context.Background())
	if err != nil {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer mint.MintDB.Rollback(context.Background(), tx)

	quote, err := mint.MintDB.GetMeltRequestById(tx, quoteId)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}

	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.GetPostMeltQuoteResponse(), nil
	}
	status, preimage, fees, err := mint.LightningBackend.CheckPayed(quote.Quote)
	if err != nil {
		if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
			return quote.GetPostMeltQuoteResponse(), nil
		}
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.LightningBackend.CheckPayed(quote.Quote). %w", err)
	}

	switch {
	case status == lightning.SETTLED:
		quote.PaymentPreimage = preimage
		quote.State = cashu.PAID
		quote.FeePaid = fees
		quote.RequestPaid = true

	case status == lightning.PENDING:
		quote.State = cashu.PENDING
	case status == lightning.FAILED:
		quote.State = cashu.UNPAID

	}

	err = mint.MintDB.AddPreimageMeltRequest(tx, quote.Quote, preimage)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote) %w", err)
	}

	err = mint.MintDB.Commit(context.Background(), tx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
	}

	return quote.GetPostMeltQuoteResponse(), nil

}
