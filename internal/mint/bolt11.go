package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/zpay32"
)

func CheckMintRequest(mint *Mint, quote cashu.MintRequestDB, invoice *zpay32.Invoice) (cashu.MintRequestDB, error) {

	status, _, err := mint.LightningBackend.CheckReceived(quote, invoice)
	if err != nil {
		return quote, fmt.Errorf("mint.VerifyLightingPaymentHappened(pool). %w", err)
	}
	switch status {
	case lightning.SETTLED:
		quote.State = cashu.PAID
	// case lightning.PENDING:
	// 	quote.State = cashu.PENDING
	case lightning.FAILED:
		quote.State = cashu.UNPAID

	}
	return quote, nil

}

func CheckMeltRequest(ctx context.Context, mint *Mint, quoteId string) (cashu.PostMeltQuoteBolt11Response, error) {

	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer func() {
		if err != nil {
			if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

	quote, err := mint.MintDB.GetMeltRequestById(tx, quoteId)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.GetMintQuoteById(pool, quoteId). %w", err)
	}

	if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
		return quote.GetPostMeltQuoteResponse(), nil
	}

	invoice, err := zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork())
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork()). %w", err)
	}

	status, preimage, fees, err := mint.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)
	if err != nil {
		if errors.Is(err, invoices.ErrInvoiceNotFound) || strings.Contains(err.Error(), "NotFound") {
			return quote.GetPostMeltQuoteResponse(), nil
		}
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.LightningBackend.CheckPayed(quote.Quote). %w", err)
	}

	switch status {
	case lightning.SETTLED:
		quote.PaymentPreimage = preimage
		quote.State = cashu.PAID
		quote.FeePaid = fees

	case lightning.PENDING:
		quote.State = cashu.PENDING
	case lightning.FAILED:
		quote.State = cashu.UNPAID

	}

	err = mint.MintDB.AddPreimageMeltRequest(tx, quote.Quote, preimage)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("database.AddPaymentPreimageToMeltRequest(pool, preimage, quote.Quote) %w", err)
	}

	err = mint.MintDB.Commit(ctx, tx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.MintDB.Commit(ctx, tx). %w", err)
	}

	return quote.GetPostMeltQuoteResponse(), nil

}
