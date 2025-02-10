package mockdb

import (
	"errors"
	"fmt"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

var DBError = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

type MockDB struct {
	Proofs        []cashu.Proof
	MeltRequest   []cashu.MeltRequestDB
	MintRequest   []cashu.MintRequestDB
	RecoverSigDB  []cashu.RecoverSigDB
	NostrAuth     []database.NostrLoginAuth
	LiquiditySwap []utils.LiquiditySwap
	Seeds         []cashu.Seed
	Config        utils.Config
	ErrorToReturn error
}

func databaseError(err error) error {
	return fmt.Errorf("%w  %w", DBError, err)
}

func (m *MockDB) GetAllSeeds() ([]cashu.Seed, error) {
	return m.Seeds, nil
}

func (m *MockDB) GetSeedsByUnit(unit cashu.Unit) ([]cashu.Seed, error) {
	var seeds []cashu.Seed
	for i := 0; i < len(m.Seeds); i++ {

		if m.Seeds[i].Unit == unit.String() {
			seeds = append(seeds, m.Seeds[i])

		}

	}
	return seeds, nil
}

func (m *MockDB) SaveNewSeed(seed cashu.Seed) error {
	m.Seeds = append(m.Seeds, seed)
	return nil
}

func (m *MockDB) SaveNewSeeds(seeds []cashu.Seed) error {
	m.Seeds = append(m.Seeds, seeds...)
	return nil
}

func (m *MockDB) UpdateSeedsActiveStatus(seeds []cashu.Seed) error {
	for i := 0; i < len(m.Seeds); i++ {
		for j := 0; j < len(seeds); j++ {
			if m.Seeds[i].Id == seeds[j].Id {
				m.Seeds[i].Active = seeds[j].Active
				break
			}
		}

	}

	return nil
}

func (m *MockDB) SaveMintRequest(request cashu.MintRequestDB) error {
	m.MintRequest = append(m.MintRequest, request)
	return nil
}

func (m *MockDB) ChangeMintRequestState(quote string, paid bool, state cashu.ACTION_STATE, minted bool) error {
	for i := 0; i < len(m.MintRequest); i++ {
		if m.MintRequest[i].Quote == quote {
			m.MintRequest[i].State = state
			m.MintRequest[i].Minted = minted
		}

	}
	return nil
}

func (m *MockDB) GetMintRequestById(id string) (cashu.MintRequestDB, error) {
	var mintRequests []cashu.MintRequestDB
	for i := 0; i < len(m.MintRequest); i++ {

		if m.MintRequest[i].Quote == id {
			mintRequests = append(mintRequests, m.MintRequest[i])

		}

	}

	return mintRequests[0], nil
}

func (m *MockDB) GetMeltRequestById(id string) (cashu.MeltRequestDB, error) {
	var meltRequests []cashu.MeltRequestDB
	for i := 0; i < len(m.MeltRequest); i++ {

		if m.MeltRequest[i].Quote == id {
			meltRequests = append(meltRequests, m.MeltRequest[i])

		}

	}

	return meltRequests[0], nil
}

func (m *MockDB) SaveMeltRequest(request cashu.MeltRequestDB) error {

	m.MeltRequest = append(m.MeltRequest, request)

	return nil

}

func (m *MockDB) AddPreimageMeltRequest(preimage string, quote string) error {
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].PaymentPreimage = preimage
		}

	}
	return nil

}
func (m *MockDB) ChangeMeltRequestState(quote string, paid bool, state cashu.ACTION_STATE, melted bool) error {
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].RequestPaid = paid
			m.MeltRequest[i].State = state
			m.MeltRequest[i].Melted = melted
		}

	}
	return nil

}

func (m *MockDB) GetProofsFromSecret(SecretList []string) ([]cashu.Proof, error) {
	var proofs []cashu.Proof
	for i := 0; i < len(SecretList); i++ {

		secret := SecretList[i]

		for j := 0; j < len(m.Proofs); j++ {

			if secret == m.Proofs[j].Secret {
				proofs = append(proofs, m.Proofs[j])

			}

		}

	}

	return proofs, nil
}

func (m *MockDB) SaveProof(proofs []cashu.Proof) error {
	m.Proofs = append(m.Proofs, proofs...)
	return nil

}

func (m *MockDB) GetProofsFromSecretCurve(Ys []string) ([]cashu.Proof, error) {
	var proofs []cashu.Proof
	for i := 0; i < len(Ys); i++ {

		secretCurve := Ys[i]

		for j := 0; j < len(m.Proofs); j++ {

			if secretCurve == m.Proofs[j].Y {
				proofs = append(proofs, m.Proofs[j])

			}

		}

	}

	return proofs, nil
}

func (m *MockDB) GetRestoreSigsFromBlindedMessages(B_ []string) ([]cashu.RecoverSigDB, error) {
	var restore []cashu.RecoverSigDB
	for i := 0; i < len(B_); i++ {

		blindMessage := B_[i]

		for j := 0; j < len(m.RecoverSigDB); j++ {

			if blindMessage == m.RecoverSigDB[j].B_ {
				restore = append(restore, m.RecoverSigDB[j])

			}

		}

	}

	return restore, nil
}

func (m *MockDB) SaveRestoreSigs(recover_sigs []cashu.RecoverSigDB) error {
	m.RecoverSigDB = append(m.RecoverSigDB, recover_sigs...)
	return nil

}

func (m *MockDB) GetProofsMintReserve() (templates.MintReserve, error) {
	var mintReserve templates.MintReserve

	for _, p := range m.Proofs {
		mintReserve.SatAmount += p.Amount
		mintReserve.Amount += 1
	}

	return mintReserve, nil
}
func (m *MockDB) GetBlindSigsMintReserve() (templates.MintReserve, error) {

	var mintReserve templates.MintReserve

	for _, p := range m.RecoverSigDB {
		mintReserve.SatAmount += p.Amount
		mintReserve.Amount += 1
	}
	return mintReserve, nil
}
