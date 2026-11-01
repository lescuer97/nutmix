package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (m *Mint) CreateMeltQuote(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request, method METHOD) (cashu.MeltRequestDB, error) {
	switch method {
	case Bolt11:
		response, err := m.createBolt11MeltQuote(ctx, meltRequest)
		if err != nil {
			return cashu.MeltRequestDB{}, fmt.Errorf("m.createBolt11MeltQuote(ctx, meltRequest). %w ", err)
		}
		return response, nil

	default:
		return cashu.MeltRequestDB{}, cashu.ErrPaymentMethodNotSupported
	}
}

func (m *Mint) createBolt11MeltQuote(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request) (cashu.MeltRequestDB, error) {
	requestData, err := m.validateBolt11MeltQuoteRequest(ctx, meltRequest)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.validateBolt11MeltQuoteRequest. %w ", err)
	}

	dbRequest, err := m.createAndStoreBolt11MeltQuote(ctx, meltRequest, requestData)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.createAndStoreBolt11MeltQuote. %w ", err)
	}
	return dbRequest, nil
}

func (m *Mint) createAndStoreBolt11MeltQuote(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request, requestData bolt11MeltReqData) (cashu.MeltRequestDB, error) {
	quoteId, err := utils.RandomHash()
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("utils.RandomHash(). %w", err)
	}

	expireTime := cashu.ExpiryTimeMinUnit(15)
	now := time.Now().Unix()
	queryFee := uint64(0)
	checkingId := quoteId
	amountToSend := requestData.Amount
	if !requestData.Internal {
		feesResponse, err := m.LightningBackend.QueryFees(meltRequest.Request, requestData.invoice, requestData.Internal, requestData.Amount)
		if err != nil {
			return cashu.MeltRequestDB{}, fmt.Errorf("m.LightningBackend.QueryFees. %w", err)
		}
		checkingId = feesResponse.CheckingId
		queryFee = feesResponse.Fees.Amount
		amountToSend = feesResponse.AmountToSend
	}
	if queryFee == math.MaxUint64 {
		return cashu.MeltRequestDB{}, cashu.ErrAmountOverflow
	}
	dbRequest := cashu.MeltRequestDB{
		Amount:          amountToSend.Amount,
		Quote:           quoteId,
		Request:         meltRequest.Request,
		Unit:            requestData.Unit.String(),
		Expiry:          expireTime,
		FeeReserve:      (queryFee + 1),
		State:           cashu.UNPAID,
		PaymentPreimage: "",
		SeenAt:          now,
		Mpp:             requestData.Mpp,
		CheckingId:      checkingId,
		FeePaid:         0,
		Melted:          false,
		Change:          nil,
	}

	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx. %w", err)
	}
	defer m.rollbackIndependent(ctx, tx)

	err = m.MintDB.SaveMeltRequest(tx, dbRequest)

	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.SaveMeltRequest(tx, dbRequest). %w", err)
	}

	err = m.MintDB.Commit(ctx, tx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.Commit(ctx, tx). %w", err)
	}
	return dbRequest, nil
}

type bolt11MeltReqData struct {
	invoice  *zpay32.Invoice
	Amount   cashu.Amount
	Unit     cashu.Unit
	Internal bool
	Mpp      bool
}

func (m *Mint) validateBolt11MeltQuoteRequest(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request) (bolt11MeltReqData, error) {
	unit, err := cashu.UnitFromString(meltRequest.Unit)
	if err != nil {
		return bolt11MeltReqData{}, errors.Join(err, cashu.ErrUnitNotSupported)
	}
	supported := m.LightningBackend.VerifyUnitSupport(unit)
	if !supported {
		return bolt11MeltReqData{}, errors.Join(err, cashu.ErrUnitNotSupported)
	}
	invoice, err := zpay32.Decode(meltRequest.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		return bolt11MeltReqData{}, fmt.Errorf(" zpay32.Decode. %w ", err)
	}

	if invoice.MilliSat == nil || uint64(*invoice.MilliSat) == 0 {
		return bolt11MeltReqData{}, cashu.ErrAmountlessInvoiceNotSupported
	}

	if m.Config.PEG_OUT_LIMIT_SATS != nil {
		if int64(*invoice.MilliSat) > (int64(*m.Config.PEG_OUT_LIMIT_SATS) * 1000) {
			return bolt11MeltReqData{}, cashu.ErrAmountOutsideLimit
		}
	}
	invoiceAmountMilisats := uint64(*invoice.MilliSat)
	cashuAmount := cashu.NewAmount(cashu.Msat, invoiceAmountMilisats)
	err = cashuAmount.To(unit)
	if err != nil {
		return bolt11MeltReqData{}, fmt.Errorf("cashuAmount.To. %w ", err)
	}
	isMpp := false
	mppAmount := cashu.NewAmount(unit, meltRequest.IsMpp())

	// if mpp is valid than change amount to mpp amount
	if mppAmount.Amount != 0 {
		if mppAmount.Amount > cashuAmount.Amount {
			return bolt11MeltReqData{}, fmt.Errorf("mpp amount is bigger than the invoice")
		}
		isMpp = true
		cashuAmount = mppAmount
		if !m.LightningBackend.ActiveMPP() {
			// TODO: Add error code multi path payments being not allowed
			return bolt11MeltReqData{}, fmt.Errorf("mpp not supported")
		}
	}
	isInternal, err := m.IsInternalTransaction(ctx, meltRequest.Request)
	if err != nil {
		return bolt11MeltReqData{}, fmt.Errorf("m.IsInternalTransaction(ctx, meltRequest.Request). %w", err)
	}

	if isMpp && isInternal {
		return bolt11MeltReqData{}, fmt.Errorf("mpp is not allowed")
	}

	return bolt11MeltReqData{Internal: isInternal, Mpp: isMpp, Amount: cashuAmount, Unit: unit, invoice: invoice}, nil
}

func (m *Mint) settleIfInternalMelt(ctx context.Context, tx pgx.Tx, meltQuote cashu.MeltRequestDB) (cashu.MeltRequestDB, error) {
	mintRequest, err := m.MintDB.GetMintRequestByRequest(tx, meltQuote.Request)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return meltQuote, nil
		}
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetMintRequestById() %w", err)
	}

	if mintRequest.Request != meltQuote.Request {
		return meltQuote, nil
	}

	if meltQuote.State == cashu.PAID {
		return meltQuote, cashu.ErrMeltAlreadyPaid
	}

	if meltQuote.Unit != mintRequest.Unit {
		return meltQuote, fmt.Errorf("unit for internal mint are not the same: %w", cashu.ErrUnitNotSupported)
	}

	if mintRequest.State != cashu.UNPAID {
		return meltQuote, fmt.Errorf("mint request has already been paid. mint state: %v", cashu.UNPAID)
	}

	meltQuote.FeePaid = 0
	meltQuote.State = cashu.PAID
	meltQuote.Melted = true

	mintRequest.State = cashu.PAID

	slog.Info(fmt.Sprintf("Settling bolt11 payment internally: %v. mintRequest: %v, %v, %v", meltQuote.Quote, mintRequest.Quote, meltQuote.Amount, meltQuote.Unit))
	err = m.MintDB.ChangeMintRequestState(tx, mintRequest.Quote, mintRequest.State, mintRequest.Minted)
	if err != nil {
		return meltQuote, fmt.Errorf("m.MintDB.ChangeMintRequestState(tx, mintRequest.Quote, mintRequest.State, mintRequest.Minted) %w", err)
	}
	err = m.MintDB.ChangeMeltRequestState(ctx, tx, meltQuote.Quote, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid)
	if err != nil {
		return meltQuote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid) %w", err)
	}

	return meltQuote, nil
}

func (m *Mint) RefreshMeltQuoteState(ctx context.Context, quoteId string) (cashu.MeltRequestDB, error) {
	initialTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer m.rollbackIndependent(ctx, initialTx)
	quote, err := m.MintDB.GetMeltRequestById(ctx, initialTx, quoteId)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			// lock not available -> treat as pending
			return quote, cashu.ErrQuoteIsPending
		}
		return quote, fmt.Errorf("m.MintDB.GetMeltRequestById(quoteId): %w", err)
	}
	pending_proofs, err := m.MintDB.GetProofsFromQuote(ctx, initialTx, quote.Quote)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
	}
	err = m.MintDB.Commit(ctx, initialTx)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.Commit(ctx, tx). %w", err)
	}

	if quote.State == cashu.PENDING {
		err = m.VerifyUnitSupport(quote.Unit)
		if err != nil {
			return quote, fmt.Errorf("m.VerifyUnitSupport(quote.Unit). %w", err)
		}

		invoice, err := zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork())

		if err != nil {
			return quote, fmt.Errorf("zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork()). %w", err)
		}

		if err := ctx.Err(); err != nil {
			return quote, err
		}
		status, preimage, feeAmount, err := m.LightningBackend.CheckPayed(ctx, quote.Quote, invoice, quote.CheckingId)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return quote, ctxErr
		}
		if err != nil {
			return quote, fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote). %w", err)
		}

		if status == lightning.SETTLED {
			observed := quote
			observed.State, observed.Melted = cashu.PAID, true
			unit, err := cashu.UnitFromString(quote.Unit)
			if err != nil {
				return quote, err
			}
			if err = feeAmount.To(unit); err != nil {
				return quote, err
			}
			observed.FeePaid, observed.PaymentPreimage = feeAmount.Amount, preimage
			settled, _, err := m.settlePaidMelt(ctx, observed, pending_proofs, "", nil, feeAmount)
			if err != nil {
				return quote, err
			}
			return settled, nil
		}
		if status == lightning.FAILED {
			quote.State = cashu.UNPAID
			failedLnTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
			}
			defer m.rollbackIndependent(ctx, failedLnTx)

			current, err := m.MintDB.GetMeltRequestById(ctx, failedLnTx, quote.Quote)
			if err != nil {
				return quote, err
			}
			if current.State != cashu.PENDING || current.CheckingId != quote.CheckingId {
				return current, nil
			}
			err = m.MintDB.ChangeMeltRequestState(ctx, failedLnTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(failedLnTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
			}

			err = m.MintDB.ReleasePendingMeltChangeOutputs(ctx, failedLnTx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ReleasePendingMeltChangeOutputs(ctx, failedLnTx, quote.Quote) %w", err)
			}
			if len(pending_proofs) > 0 {
				err = m.MintDB.DeleteProofs(ctx, failedLnTx, pending_proofs)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.DeleteProofs(failedLnTx, pending_proofs). %w", err)
				}
			}

			err = m.MintDB.Commit(ctx, failedLnTx)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.Commit(ctx, failedLnTx). %w", err)
			}
		}
	}
	if quote.State == cashu.PAID && quote.Melted {
		settled, _, settleErr := m.settlePaidMelt(ctx, quote, pending_proofs, "", nil, cashu.NewAmount(cashu.Sat, 0))
		if settleErr != nil {
			return quote, settleErr
		}
		return settled, nil
	}

	return quote, nil
}

func (m *Mint) ReconcilePendingMeltQuotes(ctx context.Context) error {
	quotes, err := m.MintDB.ListMeltQuotesForReconciliation(ctx)
	if err != nil {
		return fmt.Errorf("m.MintDB.ListMeltQuotesForReconciliation(). %w", err)
	}

	for _, quote := range quotes {
		slog.Info("Attempting to solve pending quote for", slog.Any("quote", quote))
		quoteCtx, cancel := context.WithTimeout(ctx, meltReconciliationQuoteTimeout)
		quote, err := m.RefreshMeltQuoteState(quoteCtx, quote.Quote)
		cancel()
		if err != nil {
			if errors.Is(err, lightning.ErrLNBackendEndOfLife) {
				slog.Error("pending melt quotes cannot be reconciled until the end-of-life lightning backend is replaced", slog.String("quote", quote.Quote), slog.Any("error", err))
				return nil
			}
			return fmt.Errorf("m.RefreshMeltQuoteState(ctx, quote.Quote). %w", err)
		}

		slog.Info("Melt quote state", slog.String("quote", quote.Quote), slog.String("state", string(quote.State)))
	}

	return nil
}

type bolt11MeltData struct {
	Fee          cashu.Amount
	AmountProofs cashu.Amount
	Unit         cashu.Unit
}

func (m *Mint) validateBolt11MeltInputs(meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB) (bolt11MeltData, error) {
	if len(meltRequest.Inputs) == 0 {
		return bolt11MeltData{}, fmt.Errorf("inputs or outputs are empty")
	}
	if quote.State == cashu.PENDING {
		slog.Warn("Quote is pending")
		return bolt11MeltData{}, cashu.ErrQuoteIsPending
	}

	if quote.Melted {
		slog.Info("Quote already melted", slog.String(utils.LogExtraInfo, quote.Quote))
		return bolt11MeltData{}, cashu.ErrMeltAlreadyPaid
	}

	proofsAmount, err := meltRequest.Inputs.Amount()
	if err != nil {
		return bolt11MeltData{}, errors.Join(cashu.ErrNotEnoughtProofs, err)
	}

	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return bolt11MeltData{}, err
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		if errors.Is(err, cashu.ErrAmountOverflow) {
			return bolt11MeltData{}, errors.Join(cashu.ErrNotEnoughtProofs, err)
		}
		return bolt11MeltData{}, fmt.Errorf("cashu.Fees(request.Inputs, keysets.Keysets). %w", err)
	}

	if quote.Amount > math.MaxUint64-quote.FeeReserve || quote.Amount+quote.FeeReserve > math.MaxUint64-fee {
		return bolt11MeltData{}, errors.Join(cashu.ErrNotEnoughtProofs, cashu.ErrAmountOverflow)
	}
	if proofsAmount < quote.Amount+quote.FeeReserve+fee {
		slog.Info(fmt.Sprintf("Not enought proofs to expend. Needs: %v", quote.Amount))
		return bolt11MeltData{}, fmt.Errorf("%w", cashu.ErrNotEnoughtProofs)
	}

	unit, err := cashu.UnitFromString(quote.Unit)
	if err != nil {
		return bolt11MeltData{}, fmt.Errorf("cashu.UnitFromString. %w", err)
	}

	// get unit from proofs
	proofUnit, err := checkProofsAreSameUnit(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		return bolt11MeltData{}, fmt.Errorf("m.CheckProofsAreSameUnit(proofs, keysets.Keysets). %w", err)
	}

	if len(meltRequest.Outputs) > 0 {
		// check if outputs are
		outputUnit, err := verifyOutputs(meltRequest.Outputs, keysets.Keysets)
		if err != nil {
			return bolt11MeltData{}, fmt.Errorf("m.VerifyOutputs(outputs). %w", err)
		}

		if proofUnit != outputUnit {
			return bolt11MeltData{}, fmt.Errorf("proofUnit != messageUnit. %w", cashu.ErrNotSameUnits)
		}
	}
	// validate if the proofs are correctly signed
	err = m.VerifyProofsBDHKE(meltRequest.Inputs)
	if err != nil {
		return bolt11MeltData{}, fmt.Errorf("m.VerifyProofsBDHKE(proofs). %w", err)
	}

	// Verify spending conditions - EXCLUSIVE paths following CDK pattern
	hasSigAll, err := cashu.ProofsHaveSigAll(meltRequest.Inputs)
	if err != nil {
		return bolt11MeltData{}, fmt.Errorf("cashu.ProofsHaveSigAll(inputs). %w", err)
	}

	if hasSigAll {
		// SIG_ALL path: verify all conditions match and signature is valid against combined message
		err = meltRequest.ValidateSigflag()
		if err != nil {
			return bolt11MeltData{}, fmt.Errorf("request.ValidateSigflag(). %w", err)
		}
	} else {
		// Individual verification path: verify each proof's P2PK/HTLC spend conditions
		err = cashu.VerifyProofsSpendConditions(meltRequest.Inputs)
		if err != nil {
			return bolt11MeltData{}, fmt.Errorf("cashu.VerifyProofsSpendConditions(request.Inputs). %w", err)
		}
	}
	if unit != proofUnit {
		return bolt11MeltData{}, fmt.Errorf("proofs unit are not the same as the quote. %w", cashu.ErrDifferentInputOutputUnit)
	}
	return bolt11MeltData{Fee: cashu.NewAmount(proofUnit, fee), Unit: unit, AmountProofs: cashu.NewAmount(proofUnit, proofsAmount)}, nil
}

func (m *Mint) reserveMeltInputsAndMarkPending(ctx context.Context, meltRequest cashu.PostMeltBolt11Request) (cashu.MeltRequestDB, error) {
	// check if proofs are spent and if outputs are spent
	sizeCheckTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer m.rollbackIndependent(ctx, sizeCheckTx)
	quote, err := m.MintDB.GetMeltRequestById(ctx, sizeCheckTx, meltRequest.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetMeltRequestById(preparationTx, meltRequest.Quote): %w", err)
	}
	if quote.State == cashu.PENDING {
		slog.Warn("Quote is pending")
		return cashu.MeltRequestDB{}, cashu.ErrQuoteIsPending
	}

	if quote.Melted {
		slog.Info("Quote already melted", slog.String(utils.LogExtraInfo, quote.Quote))
		return cashu.MeltRequestDB{}, cashu.ErrMeltAlreadyPaid
	}

	proofs, err := m.validateProofsUnspent(ctx, sizeCheckTx, meltRequest.Inputs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.validateProofsUnspent(sizeCheckTx, request.Inputs). %w", err)
	}

	err = m.ValidateOutputsNotSpent(ctx, sizeCheckTx, meltRequest.Outputs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.ValidateOutputsNotSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	proofs.SetPendingAndQuoteRef(quote.Quote)
	err = m.MintDB.SaveProof(ctx, sizeCheckTx, proofs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.SaveProof(sizeCheckTx, proofs). %w", err)
	}

	quote.State = cashu.PENDING
	err = m.MintDB.ChangeMeltRequestState(ctx, sizeCheckTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(preparationTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.ReserveMeltChangeOutputs(ctx, sizeCheckTx, meltRequest.Outputs, quote.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.ReserveMeltChangeOutputs(setUpTx, meltRequest.Outputs, quote.Quote) %w", err)
	}

	quote, err = m.settleIfInternalMelt(ctx, sizeCheckTx, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.settleIfInternalMelt(ctx, preparationTx, quote). %w", err)
	}

	err = m.MintDB.Commit(ctx, sizeCheckTx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.Commit(ctx, sizeCheckTx). %w", err)
	}
	return quote, nil
}

func (m *Mint) attemptBolt11MeltPayment(ctx context.Context, meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB) (cashu.MeltRequestDB, cashu.Amount, error) {
	// Commit all blind messages and proofs as pending before going over the network
	invoice, err := zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		slog.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
		return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork()) %w", err)
	}

	unit, err := cashu.UnitFromString(quote.Unit)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("cashu.UnitFromString(quote.Unit). %w", err)
	}
	amount := cashu.NewAmount(unit, quote.Amount)

	if quote.State != cashu.PAID {
		// Convert feeReserve to Amount for the lightning backend
		feeReserveAmount := cashu.NewAmount(unit, quote.FeeReserve)
		payment, err := m.LightningBackend.PayInvoice(quote, invoice, feeReserveAmount, quote.Mpp, amount)
		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN || payment.PaymentState == lightning.PENDING {
			lnTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
			}
			defer m.rollbackIndependent(ctx, lnTx)

			slog.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			slog.Debug("changing checking Id to payment checking Id", slog.String("quote.CheckingId", quote.CheckingId), slog.String("payment.CheckingId", payment.CheckingId))
			quote.CheckingId = payment.CheckingId
			err = m.MintDB.ChangeCheckingId(ctx, lnTx, quote.Quote, quote.CheckingId)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeCheckingId(lnTx, quote.Quote, quote.CheckingId): %w", err)
			}
			err = m.MintDB.Commit(ctx, lnTx)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.Commit(ctx, lnTx). %w", err)
			}

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, fee_paid, err := m.LightningBackend.CheckPayed(ctx, quote.Quote, invoice, quote.CheckingId)

			// if error on checking payement we will save as pending and returns status
			if err != nil {
				slog.Warn("Something happened while paying the invoice. Keeping proofs and quote as pending ")
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote) %w", err)
			}

			slog.Info("after check paid verification")
			// Convert fee Amount to quote's unit for storage
			convertErr := fee_paid.To(unit)
			if convertErr != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("fee_paid.To(quoteUnit). %w", convertErr)
			}
			quote.FeePaid = fee_paid.Amount

			lnStatusTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
			}
			defer m.rollbackIndependent(ctx, lnStatusTx)

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED, lightning.UNKNOWN:
				quote.State = cashu.PENDING
				// change melt request state
				err = m.MintDB.ChangeMeltRequestState(ctx, lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
				if err != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid): %w", err)
				}

			// finish failure and release the proofs
			case lightning.FAILED:
				quote.State = cashu.UNPAID
				errDb := m.MintDB.ChangeMeltRequestState(ctx, lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
				}
				errDb = m.MintDB.DeleteProofs(ctx, lnStatusTx, meltRequest.Inputs)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.DeleteProofs(lnStatusTx, meltRequest.Inputs) %w", err)
				}
				errDb = m.MintDB.ReleasePendingMeltChangeOutputs(ctx, lnStatusTx, quote.Quote)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ReleasePendingMeltChangeOutputs(ctx, lnStatusTx, quote.Quote) %w", err)
				}
			}
			err = m.MintDB.Commit(ctx, lnStatusTx)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.Commit(ctx, lnStatusTx). %w", err)
			}

			return quote, cashu.Amount{Amount: 0, Unit: unit}, nil
		}

		quote.PaymentPreimage = payment.Preimage
		// Convert fee Amount to quote's unit for storage
		convertErr := payment.PaidFee.To(unit)
		if convertErr != nil {
			return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("payment.PaidFee.To(quoteUnit). %w", convertErr)
		}
		quote.FeePaid = payment.PaidFee.Amount
		quote.State = cashu.PAID
		quote.Melted = true
		return quote, payment.PaidFee, nil
	}
	return quote, cashu.Amount{Amount: 0, Unit: unit}, nil
}

func (m *Mint) finalizePaidBolt11Melt(ctx context.Context, meltData bolt11MeltData, meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB, paidLightningFeeSat cashu.Amount) (cashu.MeltRequestDB, cashu.PostMeltQuoteBolt11Response, cashu.Proofs, error) {
	inputFee := meltData.Fee
	settled, proofs, err := m.settlePaidMelt(ctx, quote, meltRequest.Inputs, meltData.Unit.String(), &inputFee, paidLightningFeeSat)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, err
	}
	return settled, settled.GetPostMeltQuoteResponse(), proofs, nil
}

func (m *Mint) bolt11Melt(ctx context.Context, meltRequest cashu.PostMeltBolt11Request) (cashu.MeltRequestDB, cashu.PostMeltQuoteBolt11Response, error) {
	quote, err := m.RefreshMeltQuoteState(ctx, meltRequest.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.RefreshMeltQuoteState(ctx, quoteId): %w", err)
	}

	meltRequestData, err := m.validateBolt11MeltInputs(meltRequest, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.validateBolt11MeltInputs(meltRequest, quote): %w", err)
	}
	if lightning.IsBackendEndOfLife(m.LightningBackend) {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, lightning.ErrLNBackendEndOfLife
	}
	quote, err = m.reserveMeltInputsAndMarkPending(ctx, meltRequest)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.reserveMeltInputsAndMarkPending(ctx, meltRequest): %w", err)
	}

	quote, lnFee, err := m.attemptBolt11MeltPayment(ctx, meltRequest, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.attemptBolt11MeltPayment(ctx, meltRequest, quote): %w", err)
	}

	if quote.State == cashu.PAID {
		quote, response, spentProofs, err := m.finalizePaidBolt11Melt(ctx, meltRequestData, meltRequest, quote, lnFee)
		if err != nil {
			return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.finalizePaidBolt11Melt(ctx, meltRequestData, meltRequest, quote, lnFee): %w", err)
		}

		go m.Observer.SendProofsEvent(spentProofs)
		go m.Observer.SendMeltEvent(quote)

		return quote, response, nil
	}

	go m.Observer.SendProofsEvent(meltRequest.Inputs)
	go m.Observer.SendMeltEvent(quote)
	return quote, quote.GetPostMeltQuoteResponse(), nil
}

func (m *Mint) ExecuteMelt(ctx context.Context, meltRequest cashu.PostMeltBolt11Request, method METHOD) (cashu.PostMeltQuoteBolt11Response, error) {
	switch method {
	case Bolt11:
		_, response, err := m.bolt11Melt(ctx, meltRequest)
		if err != nil {
			return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.bolt11Melt. %w ", err)
		}
		return response, nil

	default:

		return cashu.PostMeltQuoteBolt11Response{}, cashu.ErrPaymentMethodNotSupported
	}
}
