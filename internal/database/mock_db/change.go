package mockdb

import (
	"github.com/lescuer97/nutmix/api/cashu"
)

func (m *MockDB) SaveMeltChange(change []cashu.BlindedMessage, quote string) error {

	for _, v := range change {
		m.MeltChange = append(m.MeltChange, cashu.MeltChange{
			B_:    v.B_,
			Id:    v.Id,
			Quote: quote,
		})

	}
	return nil
}
func (m *MockDB) GetMeltChangeByQuote(quote string) ([]cashu.MeltChange, error) {

	var change []cashu.MeltChange
	for i := 0; i < len(m.MeltChange); i++ {

		if m.MeltChange[i].Quote == quote {
			change = append(change, m.MeltChange[i])

		}

	}

	return change, nil
}

func (m *MockDB) DeleteChangeByQuote(quote string) error {
	for i := 0; i < len(m.MeltChange); i++ {

		if m.MeltChange[i].Quote == quote {
			m.MeltChange = append(m.MeltChange[:i], m.MeltChange[i+1:]...)
		}

	}

	return nil
}
