package mockdb

import (
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
)

func (m MockDB) SaveNostrAuth(auth database.NostrLoginAuth) error {
	return nil

}

func (m MockDB) UpdateNostrAuthActivation(nonce string, activated bool) error {
	return nil
}

func (m MockDB) GetNostrAuth(nonce string) (database.NostrLoginAuth, error) {
	var seeds []database.NostrLoginAuth
	for i := 0; i < len(m.NostrAuth); i++ {

		if m.Seeds[i].Unit == nonce {
			seeds = append(seeds, m.NostrAuth[i])

		}

	}
	return seeds[0], nil

}

func (m MockDB) GetMintMeltBalanceByTime(time int64) (database.MintMeltBalance, error) {
	var mintmeltbalance database.MintMeltBalance

	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].State == cashu.ISSUED || m.MeltRequest[i].State == cashu.PAID {
			mintmeltbalance.Melt = append(mintmeltbalance.Melt, m.MeltRequest[i])

		}

	}

	for j := 0; j < len(m.MeltRequest); j++ {
		if m.MintRequest[j].State == cashu.ISSUED || m.MintRequest[j].State == cashu.PAID {
			mintmeltbalance.Mint = append(mintmeltbalance.Mint, m.MintRequest[j])

		}

	}
	return mintmeltbalance, nil
}
