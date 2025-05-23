package mint

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (m *Mint) settleIfInternalMelt(tx pgx.Tx, meltQuote cashu.MeltRequestDB, logger *slog.Logger) (cashu.MeltRequestDB, error) {
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

	logger.Info(fmt.Sprintf("Settling bolt11 payment internally: %v. mintRequest: %v, %v, %v", meltQuote.Quote, mintRequest.Quote, meltQuote.Amount, meltQuote.Unit))

	err = m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.RequestPaid, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid)
	if err != nil {
		return meltQuote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, meltQuote.Quote, meltQuote.RequestPaid, meltQuote.State, meltQuote.Melted, meltQuote.FeePaid) %w", err)
	}
	err = m.MintDB.ChangeMintRequestState(tx, mintRequest.Quote, mintRequest.RequestPaid, mintRequest.State, mintRequest.Minted)
	if err != nil {
		return meltQuote, fmt.Errorf("mint.MintDB.ChangeMintRequestState(tx, mintRequest.Quote, mintRequest.RequestPaid, mintRequest.State, mintRequest.Minted): %w", err)
	}

	return meltQuote, nil
}

func (m *Mint) CheckMeltQuoteState(quoteId string) (cashu.MeltRequestDB, error) {
	ctx := context.Background()
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.MeltRequestDB{}, fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
	}

	defer m.MintDB.Rollback(ctx, tx)
	quote, err := m.MintDB.GetMeltRequestById(tx, quoteId)

	if err != nil {
		return quote, fmt.Errorf("m.MintDB.GetMeltRequestById(quoteId). %w", err)
	}
	pending_proofs, err := m.MintDB.GetProofsFromQuote(tx, quote.Quote)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.GetProofsFromQuote(quote.Quote). %w", err)
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

			changeMessages, err := m.MintDB.GetMeltChangeByQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.GetMeltChangeByQuote(tx, quote.Quote). %w", err)
			}

			keysets, err := m.Signer.GetKeys()
			if err != nil {
				return quote, fmt.Errorf("m.Signer.GetKeys(). %w", err)
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
					blindMessages = append(blindMessages, cashu.BlindedMessage{Id: v.Id, B_: v.Id})

				}
				sigs, err := m.GetChangeOutput(blindMessages, overpaidFees, quote.Unit)
				if err != nil {
					return quote, fmt.Errorf("m.GetChangeOutput(changeMessages, quote.Unit ). %w", err)
				}

				err = m.MintDB.SaveRestoreSigs(tx, sigs)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.SaveRestoreSigs(sigs) %w", err)
				}

				err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(quote.Quote) %w", err)
				}

			}

			err = m.MintDB.SetProofsState(tx, pending_proofs, cashu.PROOF_SPENT)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.SetProofsState(pending_proofs, cashu.PROOF_SPENT) %w", err)
			}

			err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

			err = m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage) %w", err)
			}

		}
		if status == lightning.FAILED {
			quote.State = cashu.UNPAID

			err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.ChangeMeltRequestState(quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.PaidFee) %w", err)
			}

			err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
			if err != nil {
				return quote, fmt.Errorf("m.MintDB.DeleteChangeByQuote(quote.Quote) %w", err)
			}
			if len(pending_proofs) > 0 {
				err = m.MintDB.DeleteProofs(tx, pending_proofs)
				if err != nil {
					return quote, fmt.Errorf("m.MintDB.DeleteProofs(tx, pending_proofs). %w", err)
				}
			}

		}

	}

	err = m.MintDB.Commit(context.Background(), tx)
	if err != nil {
		return quote, fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
	}
	return quote, nil
}

func (m *Mint) CheckPendingQuoteAndProofs(logger *slog.Logger) error {
	quotes, err := m.MintDB.GetMeltQuotesByState(cashu.PENDING)
	if err != nil {
		return fmt.Errorf("m.MintDB.GetMeltQuotesByState(cashu.PENDING). %w", err)
	}

	for _, quote := range quotes {
		logger.Info(fmt.Sprintf("Attempting to solve pending quote for: %v", quote))
		quote, err := m.CheckMeltQuoteState(quote.Quote)
		if err != nil {
			return fmt.Errorf("m.CheckMeltQuoteState(quote.Quote). %w", err)
		}

		logger.Info(fmt.Sprintf("Melt quote %v state: %v", quote.Quote, quote.State))
	}

	return nil
}

func (m *Mint) Melt(meltRequest cashu.PostMeltBolt11Request, logger *slog.Logger) (cashu.PostMeltQuoteBolt11Response, error) {
	if len(meltRequest.Inputs) == 0 {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("Outputs are empty")
	}

	quote, err := m.CheckMeltQuoteState(meltRequest.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("mint.CheckMeltQuoteState(quoteId): %w", err)
	}

	if quote.State != cashu.UNPAID {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w mint.CheckMeltQuoteState(quoteId): %w", cashu.ErrMeltAlreadyPaid, err)
	}

	ctx := context.Background()
	tx, err := m.MintDB.GetTx(ctx)
	if err != nil {
		return cashu.PostMeltQuoteBolt11Response{}, fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err)
	}
	defer m.MintDB.Rollback(ctx, tx)

	quote, err = m.MintDB.GetMeltRequestById(tx, meltRequest.Quote)

	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.GetMeltRequestById(tx, meltRequest.Quote): %w", err)
	}

	if quote.State == cashu.PENDING {
		logger.Warn("Quote is pending")
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf(" %w m.MintDB.GetMeltRequestById(tx, meltRequest.Quote): %w", cashu.ErrQuoteIsPending, err)
	}

	if quote.Melted {
		logger.Info("Quote already melted", slog.String(utils.LogExtraInfo, quote.Quote))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w quote.Melted: %w", cashu.ErrMeltAlreadyPaid, err)
	}
	keysets, err := m.Signer.GetKeys()
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.Signer.GetKeys(). %w", err)
	}

	unit, err := m.CheckProofsAreSameUnit(meltRequest.Inputs, keysets.Keysets)

	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. m.CheckProofsAreSameUnit(meltRequest.Inputs): %w", cashu.ErrUnitNotSupported, err)
	}
	if len(meltRequest.Outputs) > 0 {
		outputUnit, err := m.VerifyOutputs(meltRequest.Outputs, keysets.Keysets)
		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. m.VerifyOutputs(meltRequest.Outputs): %w", cashu.ErrUnitNotSupported, err)
		}

		if outputUnit != unit {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. Change output unit is different: ", cashu.ErrDifferentInputOutputUnit)
		}
	}

	// if there are change outputs you need to check if the outputs are valid if they have the correct unit
	if len(meltRequest.Outputs) > 0 {
		outputUnit, err := m.VerifyOutputs(meltRequest.Outputs, keysets.Keysets)
		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. m.VerifyOutputs(meltRequest.Outputs): %w", cashu.ErrUnitNotSupported, err)
		}

		if outputUnit != unit {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. Change output unit is different: ", cashu.ErrUnitNotSupported)
		}
	}

	err = m.VerifyUnitSupport(quote.Unit)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.VerifyUnitSupport(quote.Unit). %w", err)
	}
	// check for needed amount of fees
	fee, err := cashu.Fees(meltRequest.Inputs, keysets.Keysets)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("cashu.Fees(meltRequest.Inputs, mint.Keysets[unit.String()]): %w", err)
	}

	AmountProofs, SecretsList, err := utils.GetAndCalculateProofsValues(&meltRequest.Inputs)
	if err != nil {
		logger.Warn("utils.GetProofsValues(&meltRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("utils.GetAndCalculateProofsValues(&meltRequest.Inputs) %w", err)
	}

	if AmountProofs < (quote.Amount + quote.FeeReserve + uint64(fee)) {
		logger.Info(fmt.Sprintf("Not enought proofs to expend. Needs: %v", quote.Amount))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w. AmountProofs < (quote.Amount + quote.FeeReserve + uint64(fee)): %w", cashu.ErrNotEnoughtProofs, err)
	}

	// check if we know any of the proofs
	knownProofs, err := m.MintDB.GetProofsFromSecretCurve(tx, SecretsList)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.GetProofsFromSecretCurve(tx, SecretsList) %w", err)
	}

	if len(knownProofs) != 0 {
		logger.Info("Proofs already used", slog.String(utils.LogExtraInfo, fmt.Sprintf("knownproofs:  %+v", knownProofs)))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w len(knownProofs) != 0 %w", cashu.ErrProofSpent, err)
	}

	err = m.Signer.VerifyProofs(meltRequest.Inputs, meltRequest.Outputs)
	if err != nil {
		logger.Debug("Could not verify Proofs", slog.String(utils.LogExtraInfo, err.Error()))
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.Signer.VerifyProofs(meltRequest.Inputs, meltRequest.Outputs) %w", err)
	}

	invoice, err := zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork())
	if err != nil {
		logger.Info(fmt.Errorf("zpay32.Decode: %w", err).Error())
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("zpay32.Decode(quote.Request, m.LightningBackend.GetNetwork()) %w", err)
	}

	setUpTx, err := m.MintDB.SubTx(ctx, tx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SubTx(ctx, tx) %w", err)
	}
	defer m.MintDB.Rollback(ctx, setUpTx)

	// change state to pending
	meltRequest.Inputs.SetPendingAndQuoteRef(quote.Quote)
	quote.State = cashu.PENDING

	err = m.MintDB.SaveProof(setUpTx, meltRequest.Inputs)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveProof(setUpTx, meltRequest.Inputs) %w", err)
	}
	err = m.MintDB.ChangeMeltRequestState(setUpTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(setUpTx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.SaveMeltChange(setUpTx, meltRequest.Outputs, quote.Quote)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveMeltChange(setUpTx, meltRequest.Outputs, quote.Quote) %w", err)
	}
	err = m.MintDB.Commit(context.Background(), setUpTx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
	}

	quote, err = m.settleIfInternalMelt(tx, quote, logger)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
	}

	var paidLightningFeeSat uint64
	amount := cashu.Amount{
		Unit:   unit,
		Amount: quote.Amount,
	}

	log.Printf("\n amount: %+v", amount)

	if !quote.RequestPaid {

		payment, err := m.LightningBackend.PayInvoice(quote, invoice, quote.FeeReserve, quote.Mpp, amount)
		// Hardened error handling
		if err != nil || payment.PaymentState == lightning.FAILED || payment.PaymentState == lightning.UNKNOWN || payment.PaymentState == lightning.PENDING {
			logger.Warn("Possible payment failure", slog.String(utils.LogExtraInfo, fmt.Sprintf("error:  %+v. payment: %+v", err, payment)))

			logger.Debug("changing checking Id to payment checking Id", slog.String("quote.CheckingId", quote.CheckingId), slog.String("payment.CheckingId", payment.CheckingId))
			quote.CheckingId = payment.CheckingId
			err = m.MintDB.ChangeCheckingId(tx, quote.Quote, quote.CheckingId)
			if err != nil {
				logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
			}

			// if exception of lightning payment says fail do a payment status recheck.
			status, _, fee_paid, err := m.LightningBackend.CheckPayed(quote.Quote, invoice, quote.CheckingId)

			// if error on checking payement we will save as pending and returns status
			if err != nil {
				logger.Warn("Something happened while paying the invoice. Keeping proofs and quote as pending ")
				// keeps the proofs and quotes as pending in the database
				err = m.MintDB.Commit(context.Background(), tx)
				if err != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
				}
				return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.LightningBackend.CheckPayed(quote.Quote) %w", err)
			}

			logger.Info("after check payed verification")
			quote.FeePaid = fee_paid

			switch status {
			// halt transaction and return a pending state
			case lightning.PENDING, lightning.SETTLED:
				quote.State = cashu.PENDING
				// change melt request state
				err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
				if err != nil {
					logger.Error(fmt.Errorf("ModifyQuoteMeltPayStatusAndMelted: %w", err).Error())
				}

				err = m.MintDB.Commit(context.Background(), tx)
				if err != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
				}

				return quote.GetPostMeltQuoteResponse(), nil

			// finish failure and release the proofs
			case lightning.FAILED, lightning.UNKNOWN:
				quote.State = cashu.UNPAID
				errDb := m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
				}
				errDb = m.MintDB.DeleteProofs(tx, meltRequest.Inputs)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteProofs(tx, meltRequest.Inputs) %w", err)
				}
				errDb = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(tx, quote.Quote) %w", err)
				}
				errDb = m.MintDB.Commit(context.Background(), tx)
				if errDb != nil {
					return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
				}

				return quote.GetPostMeltQuoteResponse(), fmt.Errorf("%w %w. DbErr: %w", err, cashu.ErrPaymentFailed, errDb)
			}
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
	totalExpent := quote.Amount + paidLightningFeeSat + uint64(fee)
	if AmountProofs > totalExpent && len(meltRequest.Outputs) > 0 {
		overpaidFees := AmountProofs - totalExpent
		change := utils.GetMessagesForChange(overpaidFees, meltRequest.Outputs)

		blindSignatures, recoverySigsDb, err := m.Signer.SignBlindMessages(change)

		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.Signer.SignBlindMessages(change) %w", err)
		}

		err = m.MintDB.SaveRestoreSigs(tx, recoverySigsDb)

		if err != nil {
			logger.Error("recoverySigsDb", slog.String(utils.LogExtraInfo, fmt.Sprintf("%+v", recoverySigsDb)))
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SaveRestoreSigs(tx, recoverySigsDb) %w", err)

		}

		err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
		if err != nil {
			return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(tx, quote.Quote) %w", err)
		}

		response.Change = blindSignatures
	}

	err = m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.ChangeMeltRequestState(tx, quote.Quote, quote.RequestPaid, quote.State, quote.Melted, quote.FeePaid) %w", err)
	}

	err = m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.AddPreimageMeltRequest(tx, quote.Quote, quote.PaymentPreimage) %w", err)
	}

	// change proofs to spent
	meltRequest.Inputs.SetProofsState(cashu.PROOF_SPENT)

	// send proofs to database
	err = m.MintDB.SetProofsState(tx, meltRequest.Inputs, cashu.PROOF_SPENT)
	if err != nil {
		logger.Error(fmt.Errorf("Proofs: %+v", meltRequest.Inputs).Error())
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.SetProofsState(tx, meltRequest.Inputs, cashu.PROOF_SPENT) %w", err)
	}

	err = m.MintDB.DeleteChangeByQuote(tx, quote.Quote)
	if err != nil {
		logger.Info(fmt.Errorf("mint.MintDB.SaveMeltChange(meltRequest.Outputs, quote.Quote) %w", err).Error())
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.DeleteChangeByQuote(tx, quote.Quote) %w", err)
	}

	err = m.MintDB.Commit(context.Background(), tx)
	if err != nil {
		return quote.GetPostMeltQuoteResponse(), fmt.Errorf("m.MintDB.Commit(context.Background(), tx). %w", err)
	}

	m.Observer.SendMeltEvent(quote)
	return response, nil
}
