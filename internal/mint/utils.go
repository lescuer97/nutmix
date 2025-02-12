package mint

import (
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) GetChangeOutput(messages []cashu.BlindedMessage, overPaidFees uint64, unit string) ([]cashu.RecoverSigDB, error) {
	if overPaidFees > 0 && len(messages) > 0 {

		change := utils.GetMessagesForChange(overPaidFees, messages)

		_, recoverySigsDb, err := m.SignBlindedMessages(change, unit)

		if err != nil {
			return recoverySigsDb, nil
		}

		return recoverySigsDb, nil

	}
	return []cashu.RecoverSigDB{}, nil

}
