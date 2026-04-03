package mint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (m *Mint) MeltQuote(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request, method METHOD) (cashu.MeltRequestDB, error) {
	switch method {
	case Bolt11:
		response, err := m.bolt11CreateMelt(ctx, meltRequest)
		if err != nil {
			return cashu.MeltRequestDB{}, fmt.Errorf("m.bolt11CreateMelt(ctx, meltRequest). %w ", err)
		}
		return response, nil

	default:
		return cashu.MeltRequestDB{}, cashu.ErrPaymentMethodNotSupported
	}
}

func (m *Mint) bolt11CreateMelt(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request) (cashu.MeltRequestDB, error) {
	requestData, err := m.bolt11ValidateMeltRequestQuote(ctx, meltRequest)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.bolt11ValidateMeltRequestQuote. %w ", err)
	}

	dbRequest, err := m.bolt11CreateMeltRequest(ctx, meltRequest, requestData)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.bolt11CreateMeltRequest. %w ", err)
	}
	return dbRequest, nil
}
func (m *Mint) bolt11CreateMeltRequest(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request, requestData bolt11MeltReqData) (cashu.MeltRequestDB, error) {
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
	//FIXME: Add method
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
	}

	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx. %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, tx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()

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

func (m *Mint) bolt11ValidateMeltRequestQuote(ctx context.Context, meltRequest cashu.PostMeltQuoteBolt11Request) (bolt11MeltReqData, error) {
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

	if uint64(*invoice.MilliSat) == 0 {
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

func (m *Mint) settleIfInternalMelt(tx pgx.Tx, meltQuote cashu.MeltRequestDB) (cashu.MeltRequestDB, error) {
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
	err = m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid)
	if err != nil {
		return meltQuote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid) %w", err)
	}

	return meltQuote, nil
}

func (m *Mint) CheckMeltQuoteState(ctx context.Context, quoteId string) (cashu.MeltRequestDB, error) {
	initialTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, initialTx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	quote, err := m.MintDB.GetMeltRequestById(initialTx, quoteId)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			// lock not available -> treat as pending
			return quote, cashu.ErrQuoteIsPending
		}
		return quote, fmt.Errorf("m.MintDB.GetMeltRequestById(quoteId): %w", err)
	}
	pending_proofs, err := m.MintDB.GetProofsFromQuote(initialTx, quote.Quote)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
	}
	err = m.MintDB.Commit(ctx, initialTx)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
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

		status, preimage, feeAmount, err := m.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)
		if err != nil {
			return quote, fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote). %w", err)
		}

		if status == lightning.SETTLED {
			quote.State = cashu.PAID
			quote.Melted = true
			// Convert fee to quote's unit for storage
			quoteUnit, err := cashu.UnitFromString(quote.Unit)
			if err != nil {
				return quote, fmt.Errorf("cashu.UnitFromString(quote.Unit). %w", err)
			}
			convertErr := feeAmount.To(quoteUnit)
			if convertErr != nil {
				return quote, fmt.Errorf("feeAmount.To(quoteUnit). %w", convertErr)
			}
			quote.FeePaid = feeAmount.Amount
			quote.PaymentPreimage = preimage

			keysets, err := m.Signer.GetKeysets()
			if err != nil {
				return quote, fmt.Errorf("m.Signer.GetKeys(). %w", err)
			}

			settleTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, fmt.Errorf("settleTx, err := m.MintDB.GetTx(ctx). %w", err)
			}
			defer func() {
				rollbackErr := m.MintDB.Rollback(ctx, settleTx)
				if rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Warn("rollback error", slog.Any("error", rollbackErr))
					}
				}
			}()

			changeMessages, err := m.MintDB.GetMeltChangeByQuote(settleTx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.GetMeltChangeByQuote(settleTx, quote.Quote). %w", err)
			}

			fee, err := cashu.Fees(pending_proofs, keysets.Keysets)
			if err != nil {
				return quote, fmt.Errorf("cashu.Fees(pending_proofs, m.Keysets[quote.Unit]). %w", err)
			}

			totalExpent := quote.Amount + quote.FeePaid + uint64(fee)

			overpaidFees := pending_proofs.Amount() - totalExpent

			if len(changeMessages) > 0 && overpaidFees > 0 {
				var blindMessages []cashu.BlindedMessage
				for _, v := range changeMessages {
					blindMessages = append(blindMessages, cashu.BlindedMessage{Id: v.Id, B_: v.B_, Witness: "", Amount: 0})
				}
				sigs, err := m.GetChangeOutput(blindMessages, overpaidFees, quote.Unit)
				if err != nil {
					return quote, fmt.Errorf("m.GetChangeOutput(changeMessages, quote.Unit ). %w", err)
				}

				err = m.MintDB.SaveRestoreSigs(settleTx, sigs)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.SaveRestoreSigs(sigs) %w", err)
				}

				err = m.MintDB.DeleteChangeByQuote(settleTx, quote.Quote)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(quote.Quote) %w", err)
				}
			}

			err = m.MintDB.SetProofsState(settleTx, pending_proofs, cashu.PROOF_SPENT)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.SetProofsState(settleTx, pending_proofs, cashu.PROOF_SPENT) %w", err)
			}

			err = m.MintDB.ChangeMeltRequestState(settleTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

			err = m.MintDB.AddPreimageMeltRequest(settleTx, quote.Quote, quote.PaymentPreimage)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage) %w", err)
			}
			err = m.MintDB.Commit(ctx, settleTx)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.Commit(ctx, settleTx). %w", err)
			}
		}
		if status == lightning.FAILED {
			quote.State = cashu.UNPAID
			failedLnTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
			}
			defer func() {
				rollbackErr := m.MintDB.Rollback(ctx, failedLnTx)
				if rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Warn("rollback error", slog.Any("error", rollbackErr))
					}
				}
			}()

			err = m.MintDB.ChangeMeltRequestState(failedLnTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(failedLnTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
			}

			err = m.MintDB.DeleteChangeByQuote(failedLnTx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(failedLnTx, quote.Quote) %w", err)
			}
			if len(pending_proofs) > 0 {
				err = m.MintDB.DeleteProofs(failedLnTx, pending_proofs)
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

	return quote, nil
}

func (m *Mint) CheckPendingQuoteAndProofs() error {
	quotes, err := m.MintDB.GetMeltQuotesByState(cashu.PENDING)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetMeltQuotesByState(cashu.PENDING). %w", err)
	}

	for _, quote := range quotes {
		slog.Info("Attempting to solve pending quote for", slog.Any("quote", quote))
		quote, err := m.CheckMeltQuoteState(context.Background(), quote.Quote)
		if err != nil {
			return fmt.Errorf("m.CheckMeltQuoteState(ctx, quote.Quote). %w", err)
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

func (m *Mint) bolt11MeltValidate(meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB) (bolt11MeltData, error) {
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

	proofsAmount := meltRequest.Inputs.Amount()

	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return bolt11MeltData{}, err
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		return bolt11MeltData{}, fmt.Errorf("cashu.Fees(request.Inputs, keysets.Keysets). %w", err)
	}

	if proofsAmount < (quote.Amount + quote.FeeReserve + uint64(fee)) {
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
	return bolt11MeltData{Fee: cashu.NewAmount(proofUnit, uint64(fee)), Unit: unit, AmountProofs: cashu.NewAmount(proofUnit, proofsAmount)}, nil
}

func (m *Mint) validateMeltStatusAndSpent(ctx context.Context, meltRequest cashu.PostMeltBolt11Request) (cashu.MeltRequestDB, error) {
	// check if proofs are spent and if outputs are spent
	sizeCheckTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}
	defer func() {
		if err != nil {
			rollbackErr := m.MintDB.Rollback(ctx, sizeCheckTx)
			if rollbackErr != nil {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	quote, err := m.MintDB.GetMeltRequestById(sizeCheckTx, meltRequest.Quote)
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

	proofs, err := m.checkProofSpent(sizeCheckTx, meltRequest.Inputs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.checkProofSpent(sizeCheckTx, request.Inputs). %w", err)
	}

	err = m.CheckOutputSpent(sizeCheckTx, meltRequest.Outputs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.checkOutputSpent(sizeCheckTx, request.Outputs). %w", err)
	}

	proofs.SetPendingAndQuoteRef(quote.Quote)
	err = m.MintDB.SaveProof(sizeCheckTx, proofs)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.SaveProof(sizeCheckTx, proofs). %w", err)
	}

	quote.State = cashu.PENDING
	err = m.MintDB.ChangeMeltRequestState(sizeCheckTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(preparationTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.SaveMeltChange(sizeCheckTx, meltRequest.Outputs, quote.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.SaveMeltChange(setUpTx, meltRequest.Outputs, quote.Quote) %w", err)
	}

	quote, err = m.settleIfInternalMelt(sizeCheckTx, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.settleIfInternalMelt(ctx, preparationTx, quote). %w", err)
	}

	err = m.MintDB.Commit(ctx, sizeCheckTx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.Commit(ctx, sizeCheckTx). %w", err)
	}
	return quote, nil
}

func (m *Mint) bolt11PayInvoice(ctx context.Context, meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB) (cashu.MeltRequestDB, cashu.Amount, error) {
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
			defer func() {
				rollbackErr := m.MintDB.Rollback(ctx, lnTx)
				if rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Warn("rollback error", slog.Any("error", rollbackErr))
					}
				}
			}()

			slog.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			slog.Debug("changing checking Id to payment checking Id", slog.String("quote.CheckingId", quote.CheckingId), slog.String("payment.CheckingId", payment.CheckingId))
			quote.CheckingId = payment.CheckingId
			err = m.MintDB.ChangeCheckingId(lnTx, quote.Quote, quote.CheckingId)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeCheckingId(lnTx, quote.Quote, quote.CheckingId): %w", err)
			}
			err = m.MintDB.Commit(ctx, lnTx)
			if err != nil {
				return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.Commit(ctx, lnTx). %w", err)
			}

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, fee_paid, err := m.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)

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
			defer func() {
				rollbackErr := m.MintDB.Rollback(ctx, lnStatusTx)
				if rollbackErr != nil {
					if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
						slog.Warn("rollback error", slog.Any("error", rollbackErr))
					}
				}
			}()

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				quote.State = cashu.PENDING
				// change melt request state
				err = m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
				if err != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid): %w", err)
				}

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				quote.State = cashu.UNPAID
				errDb := m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
				}
				errDb = m.MintDB.DeleteProofs(lnStatusTx, meltRequest.Inputs)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.DeleteProofs(lnStatusTx, meltRequest.Inputs) %w", err)
				}
				errDb = m.MintDB.DeleteChangeByQuote(lnStatusTx, quote.Quote)
				if errDb != nil {
					return cashu.MeltRequestDB{}, cashu.Amount{}, fmt.Errorf("m.MintDB.DeleteChangeByQuote(lnStatusTx, quote.Quote) %w", err)
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

func (m *Mint) bolt11MeltBurnTokens(ctx context.Context, meltData bolt11MeltData, meltRequest cashu.PostMeltBolt11Request, quote cashu.MeltRequestDB, paidLightningFeeSat cashu.Amount) (cashu.MeltRequestDB, cashu.PostMeltQuoteBolt11Response, cashu.Proofs, error) {
	err := paidLightningFeeSat.To(meltData.Unit)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("paidLightningFeeSat.To(meltData) %w", err)
	}
	err = meltData.Fee.To(meltData.Unit)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("meltData.Fee.To(meltData) %w", err)
	}

	totalExpent := quote.Amount + paidLightningFeeSat.Amount + meltData.Fee.Amount

	var recoverySigs []cashu.RecoverSigDB
	var blindSigs []cashu.BlindSignature
	if meltData.AmountProofs.Amount > totalExpent && len(meltRequest.Outputs) > 0 {
		overpaidFees := meltData.AmountProofs.Amount - totalExpent
		change := utils.GetMessagesForChange(overpaidFees, meltRequest.Outputs)

		blindSignaturesDB, recoverySigsDb, err := m.Signer.SignBlindMessages(change)
		if err != nil {
			return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.Signer.SignBlindMessages(change) %w", err)
		}
		recoverySigs = recoverySigsDb
		blindSigs = blindSignaturesDB
	}

	paidLnxTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}
	defer func() {
		rollbackErr := m.MintDB.Rollback(ctx, paidLnxTx)
		if rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback error", slog.Any("error", rollbackErr))
			}
		}
	}()
	response := quote.GetPostMeltQuoteResponse()
	if len(recoverySigs) > 0 {
		err = m.MintDB.SaveRestoreSigs(paidLnxTx, recoverySigs)
		if err != nil {
			slog.Error("recoverySigsDb", slog.String(utils.LogExtraInfo, fmt.Sprintf("%+v", recoverySigs)))
			return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.SaveRestoreSigs(paidLnxTx, recoverySigsDb) %w", err)
		}

		err = m.MintDB.DeleteChangeByQuote(paidLnxTx, quote.Quote)
		if err != nil {
			return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.DeleteChangeByQuote(paidLnxTx, quote.Quote) %w", err)
		}
		response.Change = blindSigs
	}

	err = m.MintDB.ChangeMeltRequestState(paidLnxTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.ChangeMeltRequestState(paidLnxTx, quote.Quote, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.AddPreimageMeltRequest(paidLnxTx, quote.Quote, quote.PaymentPreimage)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.AddPreimageMeltRequest(paidLnxTx, quote.Quote, quote.PaymentPreimage) %w", err)
	}

	meltRequest.Inputs.SetProofsState(cashu.PROOF_SPENT)
	err = m.MintDB.SetProofsState(paidLnxTx, meltRequest.Inputs, cashu.PROOF_SPENT)
	if err != nil {
		slog.Error("Proofs", slog.Any("proofs", meltRequest.Inputs))
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.SetProofsState(tx, meltRequest.Inputs, cashu.PROOF_SPENT) %w", err)
	}

	err = m.MintDB.Commit(ctx, paidLnxTx)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, nil, fmt.Errorf("m.MintDB.Commit(ctx, paidLnxTx). %w", err)
	}
	return quote, response, meltRequest.Inputs, nil
}
func (m *Mint) bolt11Melt(ctx context.Context, meltRequest cashu.PostMeltBolt11Request) (cashu.MeltRequestDB, cashu.PostMeltQuoteBolt11Response, error) {
	quote, err := m.CheckMeltQuoteState(ctx, meltRequest.Quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.CheckMeltQuoteState(ctx, quoteId): %w", err)
	}

	meltRequestData, err := m.bolt11MeltValidate(meltRequest, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.bolt11MeltValidate(meltRequest, quote): %w", err)
	}
	quote, err = m.validateMeltStatusAndSpent(ctx, meltRequest)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.validateMeltStatusAndSpent(ctx, meltRequest): %w", err)
	}

	quote, lnFee, err := m.bolt11PayInvoice(ctx, meltRequest, quote)
	if err != nil {
		return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.bolt11PayInvoice(ctx, meltRequest, quote): %w", err)
	}

	if quote.State == cashu.PAID {
		quote, response, spentProofs, err := m.bolt11MeltBurnTokens(ctx, meltRequestData, meltRequest, quote, lnFee)
		if err != nil {
			return cashu.MeltRequestDB{}, cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("m.bolt11MeltBurnTokens(ctx, meltRequestData, meltRequest, quote, lnFee): %w", err)
		}

		go m.Observer.SendProofsEvent(spentProofs)
		go m.Observer.SendMeltEvent(quote)

		return quote, response, nil
	}

	go m.Observer.SendProofsEvent(meltRequest.Inputs)
	go m.Observer.SendMeltEvent(quote)
	return quote, quote.GetPostMeltQuoteResponse(), nil
}

func (m *Mint) Melt(ctx context.Context, meltRequest cashu.PostMeltBolt11Request, method METHOD) (cashu.PostMeltQuoteBolt11Response, error) {
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
