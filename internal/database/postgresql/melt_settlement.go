package postgresql

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

func (pql Postgresql) CompleteMeltSettlement(ctx context.Context, tx pgx.Tx, command database.MeltSettlementCommand) (database.MeltSettlementResult, error) {
	var result database.MeltSettlementResult
	if command.QuoteID == "" || command.Unit == "" {
		return result, database.ErrMeltSettlementState
	}
	quote, err := loadMeltRequestForUpdate(ctx, tx, command.QuoteID)
	if err != nil {
		return result, err
	}
	if quote.Unit != command.Unit || quote.CheckingId != command.CheckingID {
		return result, database.ErrMeltSettlementState
	}
	stored, err := loadProofsForUpdate(ctx, tx, "quote=$1", command.QuoteID)
	if err != nil {
		return result, err
	}
	allSpent, err := database.ValidateMeltSettlementInputs(command.QuoteID, command.Inputs, stored)
	if err != nil {
		return result, err
	}
	signed, err := pql.loadRecoverySignatures(ctx, tx, recoverySignatureSelect+`m.melt_quote_id=$1 ORDER BY m.output_position`, command.QuoteID)
	if err != nil {
		return result, err
	}
	pending, err := loadPendingMeltChangeForSettlement(ctx, tx, command.QuoteID)
	if err != nil {
		return result, err
	}
	if quote.State == cashu.PAID && quote.Melted && allSpent {
		if len(pending) != 0 {
			return result, database.ErrMeltSettlementState
		}
		quote.Change = signaturesToBlindSignatures(signed)
		return database.MeltSettlementResult{Quote: quote, Proofs: stored, AlreadyCompleted: true}, nil
	}
	if (quote.State != cashu.PENDING && quote.State != cashu.PAID) || (quote.State == cashu.PAID && !quote.Melted) || (quote.State == cashu.PENDING && quote.Melted) {
		return result, database.ErrMeltSettlementState
	}
	if allSpent {
		return result, database.ErrMeltSettlementState
	}
	if quote.State == cashu.PAID && (quote.FeePaid != command.LightningFee || quote.PaymentPreimage != command.Preimage) {
		return result, database.ErrMeltSettlementState
	}
	if len(signed) != 0 {
		return result, database.ErrMeltSettlementState
	}
	inputAmount, err := stored.Amount()
	if err != nil {
		return result, err
	}
	expense := quote.Amount
	if math.MaxUint64-expense < command.LightningFee {
		return result, cashu.ErrAmountOverflow
	}
	expense += command.LightningFee
	if math.MaxUint64-expense < command.InputFee {
		return result, cashu.ErrAmountOverflow
	}
	expense += command.InputFee
	refund := uint64(0)
	if inputAmount > expense {
		refund = inputAmount - expense
	}
	if _, err := database.ValidateMeltSettlementOutputs(pending, command.Outputs, refund); err != nil {
		return result, err
	}
	if len(command.Outputs) > 0 {
		points := make([]cashu.WrappedPublicKey, len(command.Outputs))
		for i, o := range command.Outputs {
			points[i] = o.B_
		}
		if err := lockBlindMessagePoints(ctx, tx, points); err != nil {
			return result, err
		}
	}
	for _, proof := range stored {
		tag, e := tx.Exec(ctx, `UPDATE proofs SET state = 'SPENT' WHERE quote = $1 AND state = 'PENDING' AND secret = $2 AND id = $3 AND amount = $4 AND y = $5`, command.QuoteID, proof.Secret, proof.Id, proof.Amount, proof.Y)
		if e != nil {
			return result, databaseError(e)
		}
		if tag.RowsAffected() != 1 {
			return result, database.ErrMeltSettlementState
		}
	}
	if err := pql.finalizeMeltChange(ctx, tx, command.QuoteID, pending, command.Outputs); err != nil {
		return result, err
	}
	tag, err := tx.Exec(ctx, `UPDATE melt_request SET state = 'PAID', melted = TRUE, fee_paid = $3, payment_preimage = $4 WHERE quote = $1 AND checking_id = $2 AND ((state = 'PENDING' AND melted = FALSE) OR (state = 'PAID' AND melted = TRUE AND fee_paid = $3 AND payment_preimage = $4))`, command.QuoteID, command.CheckingID, command.LightningFee, command.Preimage)
	if err != nil {
		return result, databaseError(err)
	}
	if tag.RowsAffected() != 1 {
		return result, database.ErrMeltSettlementState
	}
	quote.State, quote.Melted, quote.FeePaid, quote.PaymentPreimage = cashu.PAID, true, command.LightningFee, command.Preimage
	quote.Change = make([]cashu.BlindSignature, len(command.Outputs))
	for i, o := range command.Outputs {
		quote.Change[i] = o.GetBlindSignature()
	}
	spent := append(cashu.Proofs(nil), stored...)
	spent.SetProofsState(cashu.PROOF_SPENT)
	return database.MeltSettlementResult{Quote: quote, Proofs: spent, AlreadyCompleted: false}, nil
}

func loadPendingMeltChangeForSettlement(ctx context.Context, tx pgx.Tx, quote string) ([]cashu.MeltChange, error) {
	rows, err := tx.Query(ctx, `SELECT blinded_message AS "B_", keyset_id AS id, melt_quote_id AS quote, created_at FROM blind_messages WHERE melt_quote_id=$1 AND state='PENDING' ORDER BY output_position FOR UPDATE`, quote)
	if err != nil {
		return nil, databaseError(err)
	}
	change, err := pgx.CollectRows(rows, pgx.RowToStructByName[cashu.MeltChange])
	if err != nil {
		return nil, databaseError(err)
	}
	return change, nil
}

func signaturesToBlindSignatures(sigs []cashu.RecoverSigDB) []cashu.BlindSignature {
	out := make([]cashu.BlindSignature, len(sigs))
	for i, s := range sigs {
		out[i] = s.GetBlindSignature()
	}
	return out
}
