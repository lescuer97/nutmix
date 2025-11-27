package mint

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

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
		return meltQuote, fmt.Errorf("Unit for internal mint are not the same. %w", cashu.ErrUnitNotSupported)
	}

	if mintRequest.State != cashu.UNPAID {
		return meltQuote, fmt.Errorf("Mint request has already been paid. Mint State: %v", cashu.UNPAID)
	}

	meltQuote.FeePaid = 0
	meltQuote.State = cashu.PAID
	meltQuote.Melted = true

	mintRequest.State = cashu.PAID
	mintRequest.RequestPaid = true

	slog.Info(fmt.Sprintf("Settling bolt11 payment internally: %v. mintRequest: %v, %v, %v", meltQuote.Quote, mintRequest.Quote, meltQuote.Amount, meltQuote.Unit))
	err = m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.RequestPaid, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid)
	if err != nil {
		return meltQuote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.RequestPaid, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid) %w", err)
	}

	return meltQuote, nil
}

func (m *Mint) CheckMeltQuoteState(quoteId string) (cashu.MeltRequestDB, error) {
	ctx := context.Background()
	initialTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer m.MintDB.Rollback(ctx, initialTx)
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
	err = m.MintDB.Commit(context.Background(), initialTx)
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

		status, preimage, fee, err := m.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)
		if err != nil {
			return quote, fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote). %w", err)
		}

		if status == lightning.SETTLED {
			quote.State = cashu.PAID
			quote.FeePaid = fee
			quote.PaymentPreimage = preimage

			keysets, err := m.Signer.GetKeysets()
			if err != nil {
				return quote, fmt.Errorf("m.Signer.GetKeys(). %w", err)
			}

			settleTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, fmt.Errorf("settleTx, err := m.MintDB.GetTx(ctx). %w", err)
			}
			defer m.MintDB.Rollback(ctx, settleTx)

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
					blindMessages = append(blindMessages, cashu.BlindedMessage{Id: v.Id, B_: v.B_})
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
				return quote, fmt.Errorf("m.MintDB.SetProofsState(pending_proofs, cashu.PROOF_SPENT) %w", err)
			}

			err = m.MintDB.ChangeMeltRequestState(settleTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

			err = m.MintDB.AddPreimageMeltRequest(settleTx, quote.Quote, quote.PaymentPreimage)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage) %w", err)
			}
			err = m.MintDB.Commit(context.Background(), settleTx)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.Commit(context.Background(), settleTx). %w", err)
			}

		}
		if status == lightning.FAILED {
			quote.State = cashu.UNPAID
			failedLnTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
			}
			defer m.MintDB.Rollback(ctx, failedLnTx)

			err = m.MintDB.ChangeMeltRequestState(failedLnTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(failedLnTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
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

			err = m.MintDB.Commit(context.Background(), failedLnTx)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.Commit(context.Background(), failedLnTx). %w", err)
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
		quote, err := m.CheckMeltQuoteState(quote.Quote)
		if err != nil {
			return fmt.Errorf("m.CheckMeltQuoteState(quote.Quote). %w", err)
		}

		slog.Info("Melt quote state", slog.String("quote", quote.Quote), slog.String("state", string(quote.State)))
	}

	return nil
}

func (m *Mint) Melt(meltRequest cashu.PostMeltBolt11Request) (cashu.PostMeltQuoteBolt11Response, error) {
	if len(meltRequest.Inputs) == 0 {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("outputs are empty")
	}

	quote, err := m.CheckMeltQuoteState(meltRequest.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.CheckMeltQuoteState(quoteId): %w", err)
	}

	if quote.State != cashu.UNPAID {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w", cashu.ErrMeltAlreadyPaid)
	}

	keysets, err := m.Signer.GetKeysets()
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.Signer.GetKeys(). %w", err)
	}

	unit, err := m.CheckProofsAreSameUnit(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. m.CheckProofsAreSameUnit(meltRequest.Inputs): %w", cashu.ErrUnitNotSupported, err)
	}

	// check for needed amount of fees
	fee, err := cashu.Fees(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()]): %w", err)
	}

	AmountProofs, SecretsList, err := utils.GetAndCalculateProofsValues(&meltRequest.Inputs)
	if err != nil {
		slog.Warn("utils.GetProofsValues(&meltRequest.Inputs)", slog.Any("error", err))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("utils.GetAndCalculateProofsValues(&meltRequest.Inputs) %w", err)
	}

	if AmountProofs < (quote.Amount + quote.FeeReserve + uint64(fee)) {
		slog.Info(fmt.Sprintf("Not enought proofs to expend. Needs: %v", quote.Amount))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w", cashu.ErrNotEnoughtProofs)
	}

	log.Printf("\n meltRequest.Inputs: %+v", meltRequest.Inputs)

	// Verify spending conditions
	hasSigAll, err := cashu.ProofsHaveSigAll(meltRequest.Inputs)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("cashu.ProofsHaveSigAll(meltRequest.Inputs) %w", err)
	}

	if hasSigAll {
		// SIG_ALL path: verify all conditions match and signature is valid against combined message
		err = meltRequest.ValidateSigflag()
		if err != nil {
			slog.Debug("meltRequest.ValidateSigflag()", slog.String(utils.LogExtraInfo, err.Error()))
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("meltRequest.ValidateSigflag() %w", err)
		}
	} else {
		// Individual verification path: verify each proof's P2PK/HTLC spend conditions
		err = m.VerifyProofsSpendConditions(meltRequest.Inputs)
		if err != nil {
			slog.Debug("m.VerifyProofsSpendConditions(meltRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.VerifyProofsSpendConditions(meltRequest.Inputs) %w", err)
		}
	}

	// Always verify BDHKE cryptographic signatures (regardless of SIG_ALL)
	err = m.VerifyProofsBDHKE(meltRequest.Inputs)
	if err != nil {
		slog.Debug("m.VerifyProofsBDHKE(meltRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.VerifyProofsBDHKE(meltRequest.Inputs) %w", err)
	}

	ctx := context.Background()
	preparationTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}
	defer m.MintDB.Rollback(ctx, preparationTx)

	quote, err = m.MintDB.GetMeltRequestById(preparationTx, meltRequest.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.GetMeltRequestById(preparationTx, meltRequest.Quote): %w", err)
	}
	err = m.VerifyUnitSupport(quote.Unit)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.VerifyUnitSupport(quote.Unit). %w", err)
	}

	if quote.State == cashu.PENDING {
		slog.Warn("Quote is pending")
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w", cashu.ErrQuoteIsPending)
	}

	if quote.Melted {
		slog.Info("Quote already melted", slog.String(utils.LogExtraInfo, quote.Quote))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w", cashu.ErrMeltAlreadyPaid)
	}

	// check if we know any of the proofs
	knownProofs, err := m.MintDB.GetProofsFromSecretCurve(preparationTx, SecretsList)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(preparationTx, SecretsList) %w", err)
	}

	if len(knownProofs) != 0 {
		slog.Debug("Proofs already used", slog.String(utils.LogExtraInfo, fmt.Sprintf("knownproofs:  %+v", knownProofs)))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w: len(knownProofs) != 0", cashu.ErrProofSpent)
	}
	if len(meltRequest.Outputs) > 0 {
		outputUnit, err := m.VerifyOutputs(preparationTx, meltRequest.Outputs, keysets.Keysets)
		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. m.VerifyOutputs(meltRequest.Outputs): %w", cashu.ErrUnitNotSupported, err)
		}

		if outputUnit != unit {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. Change output unit is different: ", cashu.ErrDifferentInputOutputUnit)
		}
	}
	// change state to pending
	meltRequest.Inputs.SetPendingAndQuoteRef(quote.Quote)
	quote.State = cashu.PENDING

	err = m.MintDB.SaveProof(preparationTx, meltRequest.Inputs)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveProof(preparationTx, meltRequest.Inputs) %w", err)
	}
	err = m.MintDB.ChangeMeltRequestState(preparationTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(preparationTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.SaveMeltChange(preparationTx, meltRequest.Outputs, quote.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveMeltChange(setUpTx, meltRequest.Outputs, quote.Quote) %w", err)
	}

	quote, err = m.settleIfInternalMelt(preparationTx, quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
	}

	err = m.MintDB.Commit(context.Background(), preparationTx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), preparationTx). %w", err)
	}

	// Commit all blind messages and proofs as pending before going over the network
	invoice, err := zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		slog.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork()) %w", err)
	}

	var paidLightningFeeSat uint64
	amount := cashu.Amount{
		Unit:   unit,
		Amount: quote.Amount,
	}

	if !quote.RequestPaid {

		payment, err := m.LightningBackend.PayInvoice(quote, invoice, quote.FeeReserve, quote.Mpp, amount)
		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN || payment.PaymentState == lightning.PENDING {
			lnTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
			}
			defer m.MintDB.Rollback(ctx, lnTx)

			slog.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			slog.Debug("changing checking Id to payment checking Id", slog.String("quote.CheckingId", quote.CheckingId), slog.String("payment.CheckingId", payment.CheckingId))
			quote.CheckingId = payment.CheckingId
			err = m.MintDB.ChangeCheckingId(lnTx, quote.Quote, quote.CheckingId)
			if err != nil {
				slog.Error(fmt.Errorf("m.MintDB.ChangeCheckingId(lnTx, quote.Quote, quote.CheckingId): %w", err).Error())
			}
			err = m.MintDB.Commit(context.Background(), lnTx)
			if err != nil {
				return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), lnTx). %w", err)
			}

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, fee_paid, err := m.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)

			// if error on checking payement we will save as pending and returns status
			if err != nil {
				slog.Warn("Something happened while paying the invoice. Keeping proofs and quote as pending ")
				return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote) %w", err)
			}

			slog.Info("after check payed verification")
			quote.FeePaid = fee_paid

			lnStatusTx, err := m.MintDB.GetTx(ctx)
			if err != nil {
				return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
			}
			defer m.MintDB.Rollback(ctx, lnStatusTx)

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				quote.State = cashu.PENDING
				// change melt request state
				err = m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
				if err != nil {
					slog.Error(fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid): %w", err).Error())
				}

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				quote.State = cashu.UNPAID
				errDb := m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(lnStatusTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
				}
				errDb = m.MintDB.DeleteProofs(lnStatusTx, meltRequest.Inputs)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteProofs(lnStatusTx, meltRequest.Inputs) %w", err)
				}
				errDb = m.MintDB.DeleteChangeByQuote(lnStatusTx, quote.Quote)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(lnStatusTx, quote.Quote) %w", err)
				}
			}
			err = m.MintDB.Commit(context.Background(), lnStatusTx)
			if err != nil {
				return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), lnStatusTx). %w", err)
			}

			return quote.GetPostMeltQuoteResponse(), nil
		}
		quote.PaymentPreimage = payment.Preimage
		paidLightningFeeSat = uint64(payment.PaidFeeSat)
		quote.FeePaid = paidLightningFeeSat
		quote.RequestPaid = true
		quote.State = cashu.PAID
		quote.Melted = true
	}

	response := quote.GetPostMeltQuoteResponse()

	// if fees where lower than expected return sats to the user

	//  if total expent is lower that the amount of proofs that where given
	//  change is returned
	paidLnxTx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}
	defer m.MintDB.Rollback(ctx, paidLnxTx)
	totalExpent := quote.Amount + paidLightningFeeSat + uint64(fee)
	if AmountProofs > totalExpent && len(meltRequest.Outputs) > 0 {
		overpaidFees := AmountProofs - totalExpent
		change := utils.GetMessagesForChange(overpaidFees, meltRequest.Outputs)

		blindSignatures, recoverySigsDb, err := m.Signer.SignBlindMessages(change)

		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.Signer.SignBlindMessages(change) %w", err)
		}

		err = m.MintDB.SaveRestoreSigs(paidLnxTx, recoverySigsDb)
		if err != nil {
			slog.Error("recoverySigsDb", slog.String(utils.LogExtraInfo, fmt.Sprintf("%+v", recoverySigsDb)))
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveRestoreSigs(paidLnxTx, recoverySigsDb) %w", err)

		}

		err = m.MintDB.DeleteChangeByQuote(paidLnxTx, quote.Quote)
		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(paidLnxTx, quote.Quote) %w", err)
		}

		response.Change = blindSignatures
	}

	err = m.MintDB.ChangeMeltRequestState(paidLnxTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(paidLnxTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.AddPreimageMeltRequest(paidLnxTx, quote.Quote, quote.PaymentPreimage)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.AddPreimageMeltRequest(paidLnxTx, quote.Quote, quote.PaymentPreimage) %w", err)
	}

	// change proofs to spent
	meltRequest.Inputs.SetProofsState(cashu.PROOF_SPENT)
	// send proofs to database
	err = m.MintDB.SetProofsState(paidLnxTx, meltRequest.Inputs, cashu.PROOF_SPENT)
	if err != nil {
		slog.Error("Proofs", slog.Any("proofs", meltRequest.Inputs))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SetProofsState(tx, meltRequest.Inputs, cashu.PROOF_SPENT) %w", err)
	}

	err = m.MintDB.DeleteChangeByQuote(paidLnxTx, quote.Quote)
	if err != nil {
		slog.Info("mint.MintDB.SaveMeltChange(meltRequest.Outputs, quote.Quote)", slog.Any("error", err))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(tx, quote.Quote) %w", err)
	}

	err = m.MintDB.Commit(context.Background(), paidLnxTx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), paidLnxTx). %w", err)
	}

	go m.Observer.SendProofsEvent(meltRequest.Inputs)
	go m.Observer.SendMeltEvent(quote)
	return response, nil
}
