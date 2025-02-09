package mint

import "github.com/lescuer97/nutmix/api/cashu"

func GetMessagesForChange(overpaidFees uint64, outputs []cashu.BlindedMessage) []cashu.BlindedMessage {
	amounts := cashu.AmountSplit(overpaidFees)
	// if there are more outputs then amount to change.
	// we size down the total amount of blind messages
	switch {
	case len(amounts) > len(outputs):
		for i := range outputs {
			outputs[i].Amount = amounts[i]
		}

	default:
		outputs = outputs[:len(amounts)]

		for i := range outputs {
			outputs[i].Amount = amounts[i]
		}

	}
	return outputs
}

func (m *Mint) GetChangeOutput(messages []cashu.BlindedMessage, overPaidFees uint64, unit string) ([]cashu.RecoverSigDB, error) {
	if overPaidFees > 0 && len(messages) > 0 {

		change := GetMessagesForChange(overPaidFees, messages)

		_, recoverySigsDb, err := m.SignBlindedMessages(change, unit)

		if err != nil {
			return recoverySigsDb, nil
		}

		return recoverySigsDb, nil

	}
	return []cashu.RecoverSigDB{}, nil

}
