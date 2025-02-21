package mint

import (
	"fmt"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) GetChangeOutput(messages []cashu.BlindedMessage, overPaidFees uint64, unit string) ([]cashu.RecoverSigDB, error) {
	if overPaidFees > 0 && len(messages) > 0 {

		change := utils.GetMessagesForChange(overPaidFees, messages)

		_, recoverySigsDb, err := m.Signer.SignBlindMessages(change)

		if err != nil {
			return recoverySigsDb, fmt.Errorf("m.Signer.SignBlindMessages(change). %w", err)
		}

		return recoverySigsDb, nil

	}
	return []cashu.RecoverSigDB{}, nil

}
