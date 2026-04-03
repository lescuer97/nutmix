package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (m *Mint) CreateMintQuote(ctx context.Context, request cashu.PostMintQuoteBolt11Request, method METHOD) (cashu.PostMintQuoteBolt11Response, error) {
	unit, err := m.validateMintConfiguration(request)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf("m.validateMintConfiguration(). %w", err)
	}
	switch method {
	case Bolt11:
		supported := m.LightningBackend.VerifyUnitSupport(unit)
		if !supported {
			return cashu.PostMintQuoteBolt11Response{}, errors.Join(err, cashu.ErrUnitNotSupported)
		}
		response, err := m.bolt11GenerateMintQuote(ctx, request, unit)
		if err != nil {
			return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf("m.generateBolt11MintRequest(request,unit). %w", err)
		}
		return response, nil

	default:
		return cashu.PostMintQuoteBolt11Response{}, cashu.ErrPaymentMethodNotSupported
	}
}
func (m *Mint) validateMintConfiguration(request cashu.PostMintQuoteBolt11Request) (cashu.Unit, error) {
	if request.Amount == 0 {
		return cashu.Sat, fmt.Errorf("amount empty")
	}

	if m.Config.PEG_OUT_ONLY {
		return cashu.Sat, cashu.ErrMintintDisabled
	}

	if m.Config.PEG_IN_LIMIT_SATS != nil {
		if request.Amount > uint64(*m.Config.PEG_IN_LIMIT_SATS) {
			slog.Info("Mint amount over the limit", slog.Uint64("amount", request.Amount))

			return cashu.Sat, cashu.ErrAmountOutsideLimit
		}
	}

	unit, err := cashu.UnitFromString(request.Unit)
	if err != nil {
		return cashu.Sat, errors.Join(err, cashu.ErrUnitNotSupported)
	}

	return unit, nil
}

func (m *Mint) bolt11GenerateMintQuote(ctx context.Context, request cashu.PostMintQuoteBolt11Request, unit cashu.Unit) (cashu.PostMintQuoteBolt11Response, error) {
	resInvoice, err := m.LightningBackend.RequestInvoice(cashu.NewAmount(unit, request.Amount), request.Description)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.LightningBackend.RequestInvoice. %w", err)
	}
	quoteId, err := utils.RandomHash()
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" utils.RandomHash() %w ", err)
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	now := time.Now().Unix()

	mintRequestDB := cashu.MintRequestDB{
		Quote:       quoteId,
		Expiry:      expireTime,
		Unit:        unit.String(),
		State:       cashu.UNPAID,
		SeenAt:      now,
		Amount:      &request.Amount,
		Pubkey:      request.Pubkey,
		Description: request.Description,
		Request:     resInvoice.PaymentRequest,
		CheckingId:  resInvoice.CheckingId,
		Minted:      false,
	}
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

	err = m.MintDB.SaveMintRequest(tx, mintRequestDB)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.SaveMintRequest(tx, mintRequestDB). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.Commit(ctx, tx). %w", err)
	}

	return mintRequestDB.PostMintQuoteBolt11Response(), nil
}

// FIXME: the method should be inside the MintRequestDB struct. this needs to change in the db and add a migration
func (m *Mint) MintQuoteStatus(ctx context.Context, quoteId string, method METHOD) (cashu.PostMintQuoteBolt11Response, error) {
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	quote, err := m.MintDB.GetMintRequestById(tx, quoteId)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.GetMintRequestById(tx, quoteId). %w", err)
	}
	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf(" m.MintDB.Commit(ctx, tx). %w", err)
	}
	switch method {
	case Bolt11:
		if quote.State == cashu.PAID || quote.State == cashu.ISSUED {
			return quote.PostMintQuoteBolt11Response(), nil
		}
		bolt11Quote, err := m.bolt11CheckQuote(ctx, quote, method)
		if err != nil {
			return cashu.PostMintQuoteBolt11Response{}, fmt.Errorf("m.bolt11CheckQuote(ctx, quote, method). %w", err)
		}
		return bolt11Quote.PostMintQuoteBolt11Response(), nil

	default:
		return cashu.PostMintQuoteBolt11Response{}, cashu.ErrPaymentMethodNotSupported
	}
}

// FIXME: the method should be inside the MintRequestDB struct. this needs to change in the db and add a migration
func (m *Mint) bolt11CheckQuote(ctx context.Context, request cashu.MintRequestDB, method METHOD) (cashu.MintRequestDB, error) {
	if method != Bolt11 {
		return cashu.MintRequestDB{}, fmt.Errorf("request method is not BOLT11")
	}
	invoice, err := zpay32.Decode(request.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("zpay32.Decode(request.Request, m.LightningBackend.GetNetwork()). %w", err)
	}

	status, _, err := m.LightningBackend.CheckReceived(request, invoice)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("mint.VerifyLightingPaymentHappened(pool). %w", err)
	}
	stateChangeTX, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf(" m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, stateChangeTX)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

	switch status {
	case lightning.SETTLED:
		err = m.MintDB.ChangeMintRequestState(stateChangeTX, request.Quote, cashu.PAID, request.Minted)
		if err != nil {
			return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.ChangeMintRequestState(stateChangeTX, request.Quote, cashu.PAID, request.Minted). %w", err)
		}
	case lightning.PENDING:
		// quote.State = cashu.PENDING
	case lightning.FAILED:
		err = m.MintDB.ChangeMintRequestState(stateChangeTX, request.Quote, cashu.UNPAID, request.Minted)
		if err != nil {
			return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.ChangeMintRequestState(stateChangeTX, request.Quote, cashu.UNPAID, request.Minted). %w", err)
		}
	}

	quote, err := m.MintDB.GetMintRequestById(stateChangeTX, request.Quote)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.GetMintRequestById(stateChangeTX, request.Quote). %w", err)
	}
	err = m.MintDB.Commit(ctx, stateChangeTX)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf(" m.MintDB.Commit(ctx, tx). %w", err)
	}

	return quote, nil
}

func (m *Mint) Mint(ctx context.Context, request cashu.PostMintBolt11Request, method METHOD) (cashu.PostMintBolt11Response, error) {
	mintReq, err := m.mintRequestValidate(ctx, request)
	if err != nil {
		return cashu.PostMintBolt11Response{}, fmt.Errorf(" mintRequestValidate(ctx, request). %w", err)
	}
	switch method {
	case Bolt11:
		response, err := m.bolt11Mint(ctx, request, mintReq, method)
		if err != nil {
			return cashu.PostMintBolt11Response{}, fmt.Errorf("m.bolt11Mint. %w", err)
		}
		return response, nil

	default:
		return cashu.PostMintBolt11Response{}, cashu.ErrPaymentMethodNotSupported
	}
}

// takes the general values of the minting process and analyses them even before going to the method branching.
func (m *Mint) mintRequestValidate(ctx context.Context, request cashu.PostMintBolt11Request) (cashu.MintRequestDB, error) {
	preparationTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf(" m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, preparationTx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	quote, err := m.MintDB.GetMintRequestById(preparationTx, request.Quote)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.GetMintRequestById(preparationTx, request.Quote). %w", err)
	}
	err = m.MintDB.Commit(ctx, preparationTx)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf(" m.MintDB.Commit(ctx, tx). %w", err)
	}
	err = m.validateMintStatusAndAuth(request, quote)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf(" m.bolt11ValidateMint(ctx, request, quote). %w", err)
	}
	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.Signer.GetKeysets(). %w", err)
	}

	outputUnit, err := verifyOutputs(request.Outputs, keysets.Keysets)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("verifyOutputs(request.Outputs, keysets.Keysets). %w", err)
	}

	if outputUnit.String() != quote.Unit {
		return cashu.MintRequestDB{}, cashu.ErrDifferentInputOutputUnit
	}

	// check if proofs are spent and if outputs are spent
	sizeCheckTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		if err != nil {
			rollbackErr := m.MintDB.Rollback(ctx, sizeCheckTx)
			if rollbackErr != nil {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	err = m.CheckOutputSpent(sizeCheckTx, request.Outputs)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}
	err = m.MintDB.Commit(ctx, sizeCheckTx)
	if err != nil {
		return cashu.MintRequestDB{}, fmt.Errorf("m.MintDB.Commit(ctx, sizeCheckTx). %w", err)
	}
	return quote, nil
}

// FIXME: the method should be inside the MintRequestDB struct. this needs to change in the db and add a migration
func (m *Mint) bolt11Mint(ctx context.Context, request cashu.PostMintBolt11Request, mintReq cashu.MintRequestDB, method METHOD) (cashu.PostMintBolt11Response, error) {
	if method != Bolt11 {
		return cashu.PostMintBolt11Response{}, fmt.Errorf("request method is not BOLT11")
	}

	unit, err := cashu.UnitFromString(mintReq.Unit)
	if err != nil {
		return cashu.PostMintBolt11Response{}, fmt.Errorf("cashu.UnitFromString(mintReq.Unit) %w", err)
	}

	supported := m.LightningBackend.VerifyUnitSupport(unit)
	if !supported {
		return cashu.PostMintBolt11Response{}, fmt.Errorf(" m.LightningBackend.VerifyUnitSupport(unit). %w. %w", err, cashu.ErrUnitNotSupported)
	}

	invoice, err := zpay32.Decode(mintReq.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		return cashu.PostMintBolt11Response{}, fmt.Errorf("zpay32.Decode(mintRequestDB.Request, mint.LightningBackend.GetNetwork()). %w", err)
	}
	cashuBlindMessage := cashu.NewAmount(unit, request.Outputs.Amount())
	err = cashuBlindMessage.To(cashu.Msat)
	if err != nil {
		return cashu.PostMintBolt11Response{}, err
	}

	// Mint outputs must match the invoice amount exactly.
	if uint64(*invoice.MilliSat) != cashuBlindMessage.Amount {
		slog.Info("mismatched amount of milisats", slog.Int("invoice_milisats", int(*invoice.MilliSat)), slog.Int("requested_milisats", int(cashuBlindMessage.Amount)))
		return cashu.PostMintBolt11Response{}, cashu.ErrAmountNotEqualToInvoice
	}

	if mintReq.State != cashu.PAID {
		mintReq, err = m.bolt11CheckQuote(ctx, mintReq, method)
		if err != nil {
			return cashu.PostMintBolt11Response{}, fmt.Errorf("m.bolt11CheckQuote(ctx, quote, method). %w", err)
		}
	}

	if mintReq.State != cashu.PAID {
		return cashu.PostMintBolt11Response{}, cashu.ErrRequestNotPaid
	}

	blindSigs, err := m.signAndSaveSigs(ctx, request, mintReq)
	if err != nil {
		return cashu.PostMintBolt11Response{}, err
	}
	return cashu.PostMintBolt11Response{Signatures: blindSigs}, nil
}

func (m *Mint) signAndSaveSigs(ctx context.Context, request cashu.PostMintBolt11Request, mintRequestDB cashu.MintRequestDB) ([]cashu.BlindSignature, error) {
	blindedSignatures, recoverySigsDb, err := m.Signer.SignBlindMessages(request.Outputs)
	if err != nil {
		return nil, fmt.Errorf("m.Signer.SignBlindMessages(request.Outputs) %w", err)
	}
	mintRequestDB.State = cashu.ISSUED
	mintRequestDB.Minted = true
	afterMintingTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return nil, fmt.Errorf(" m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, afterMintingTx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	err = m.MintDB.ChangeMintRequestState(afterMintingTx, mintRequestDB.Quote, mintRequestDB.State, mintRequestDB.Minted)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.ChangeMintRequestState. %w", err)
	}

	slog.Debug(fmt.Sprintf("Saving restore sigs for quote: id %v", mintRequestDB.Quote))
	err = m.MintDB.SaveRestoreSigs(afterMintingTx, recoverySigsDb)
	if err != nil {
		return nil, fmt.Errorf("m.MintDB.SaveRestoreSigs. %w", err)
	}
	err = m.MintDB.Commit(ctx, afterMintingTx)
	if err != nil {
		return nil, fmt.Errorf(" m.MintDB.Commit(ctx, tx). %w", err)
	}
	go m.Observer.SendMintEvent(mintRequestDB)
	return blindedSignatures, nil
}

func (m *Mint) validateMintStatusAndAuth(request cashu.PostMintBolt11Request, mintRequestDB cashu.MintRequestDB) error {
	if mintRequestDB.Minted {
		return cashu.ErrMintRequestAlreadyIssued
	}

	if mintRequestDB.Pubkey.PublicKey != nil {
		valid, err := request.VerifyPubkey(mintRequestDB.Pubkey.PublicKey)
		if err != nil {
			return fmt.Errorf("request.VerifyPubkey(mintRequestDB.Pubkey.PublicKey). %w", err)
		}

		if !valid {
			return fmt.Errorf("invalid pubkey signature. %w", err)
		}
	}
	return nil
}
