package mockdb

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
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
	MeltChange    []cashu.MeltChange
	Seeds         []cashu.Seed
	AuthUser      []database.AuthUser
	Config        utils.Config
	ErrorToReturn error
}

func databaseError(err error) error {
	return errors.Join(DBError, err)
}

func (m *MockDB) GetAllSeeds() ([]cashu.Seed, error) {
	return m.Seeds, nil
}
func (m *MockDB) GetTx(ctx context.Context) (pgx.Tx, error) {
	return &pgxpool.Tx{}, nil
}
func (m *MockDB) SubTx(ctx context.Context, tx pgx.Tx) (pgx.Tx, error) {
	return &pgxpool.Tx{}, nil
}
func (m *MockDB) Commit(ctx context.Context, tx pgx.Tx) error {
	return nil
}
func (m *MockDB) Rollback(ctx context.Context, tx pgx.Tx) error {
	return nil
}

func (m *MockDB) GetSeedsByUnit(tx pgx.Tx, unit cashu.Unit) ([]cashu.Seed, error) {
	var seeds []cashu.Seed
	for i := 0; i < len(m.Seeds); i++ {

		if m.Seeds[i].Unit == unit.String() {
			seeds = append(seeds, m.Seeds[i])

		}

	}
	return seeds, nil
}

func (m *MockDB) SaveNewSeed(tx pgx.Tx, seed cashu.Seed) error {
	m.Seeds = append(m.Seeds, seed)
	return nil
}

func (m *MockDB) SaveNewSeeds(seeds []cashu.Seed) error {
	m.Seeds = append(m.Seeds, seeds...)
	return nil
}

func (m *MockDB) UpdateSeedsActiveStatus(tx pgx.Tx, seeds []cashu.Seed) error {
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

func (m *MockDB) SaveMintRequest(tx pgx.Tx, request cashu.MintRequestDB) error {
	m.MintRequest = append(m.MintRequest, request)
	return nil
}

func (m *MockDB) ChangeMintRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, minted bool) error {
	for i := 0; i < len(m.MintRequest); i++ {
		if m.MintRequest[i].Quote == quote {
			m.MintRequest[i].State = state
			m.MintRequest[i].Minted = minted
		}

	}
	return nil
}

func (m *MockDB) GetMintRequestById(tx pgx.Tx, id string) (cashu.MintRequestDB, error) {
	var mintRequests []cashu.MintRequestDB
	for i := 0; i < len(m.MintRequest); i++ {

		if m.MintRequest[i].Quote == id {
			mintRequests = append(mintRequests, m.MintRequest[i])

		}

	}
	if len(mintRequests) == 0 {
		return cashu.MintRequestDB{}, pgx.ErrNoRows
	}

	return mintRequests[0], nil
}
func (m *MockDB) GetMintRequestByRequest(tx pgx.Tx, request string) (cashu.MintRequestDB, error) {
	var mintRequests []cashu.MintRequestDB
	for i := 0; i < len(m.MintRequest); i++ {
		if m.MintRequest[i].Request == request {
			mintRequests = append(mintRequests, m.MintRequest[i])
		}
	}

	if len(mintRequests) == 0 {
		return cashu.MintRequestDB{}, pgx.ErrNoRows
	}

	return mintRequests[0], nil
}

func (m *MockDB) GetMeltRequestById(tx pgx.Tx, id string) (cashu.MeltRequestDB, error) {
	var meltRequests []cashu.MeltRequestDB
	for i := 0; i < len(m.MeltRequest); i++ {

		if m.MeltRequest[i].Quote == id {
			meltRequests = append(meltRequests, m.MeltRequest[i])
		}
	}
	if len(meltRequests) == 0 {
		return cashu.MeltRequestDB{}, pgx.ErrNoRows
	}

	return meltRequests[0], nil
}
func (m *MockDB) GetMeltQuotesByState(state cashu.ACTION_STATE) ([]cashu.MeltRequestDB, error) {
	var meltRequests []cashu.MeltRequestDB
	for i := 0; i < len(m.MeltRequest); i++ {

		if m.MeltRequest[i].State == state {
			meltRequests = append(meltRequests, m.MeltRequest[i])

		}
	}
	if len(meltRequests) == 0 {
		return meltRequests, pgx.ErrNoRows
	}

	return meltRequests, nil
}

func (m *MockDB) SaveMeltRequest(tx pgx.Tx, request cashu.MeltRequestDB) error {

	m.MeltRequest = append(m.MeltRequest, request)

	return nil

}

func (m *MockDB) AddPreimageMeltRequest(tx pgx.Tx, preimage string, quote string) error {
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].PaymentPreimage = preimage
		}

	}
	return nil

}
func (m *MockDB) ChangeMeltRequestState(tx pgx.Tx, quote string, paid bool, state cashu.ACTION_STATE, melted bool, paid_fee uint64) error {
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].RequestPaid = paid
			m.MeltRequest[i].State = state
			m.MeltRequest[i].Melted = melted
			m.MeltRequest[i].FeePaid = paid_fee
		}

	}
	return nil
}
func (m *MockDB) ChangeCheckingId(tx pgx.Tx, quote string, checking_id string) error {
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].CheckingId = checking_id
		}

	}
	return nil
}

func (m *MockDB) GetProofsFromSecret(tx pgx.Tx, SecretList []string) (cashu.Proofs, error) {
	var proofs cashu.Proofs
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
func (m *MockDB) GetProofsFromQuote(tx pgx.Tx, quote string) (cashu.Proofs, error) {
	var proofs cashu.Proofs

	for j := 0; j < len(m.Proofs); j++ {

		if m.Proofs[j].Quote != nil {
			if quote == *m.Proofs[j].Quote {
				proofs = append(proofs, m.Proofs[j])

			}
		}
	}

	return proofs, nil
}

func (m *MockDB) SaveProof(tx pgx.Tx, proofs []cashu.Proof) error {
	m.Proofs = append(m.Proofs, proofs...)
	return nil

}

func (m *MockDB) GetProofsFromSecretCurve(tx pgx.Tx, Ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	var proofs cashu.Proofs
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

func (m *MockDB) DeleteProofs(tx pgx.Tx, proofs cashu.Proofs) error {
	for i := 0; i < len(m.Proofs); i++ {
		for j := 0; j < len(proofs); j++ {
			if proofs[j].Y == m.Proofs[i].Y {
				m.Proofs = append(m.Proofs[:i], m.Proofs[i+1:]...)
			}
		}
	}

	return nil
}

func (m *MockDB) SetProofsState(tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	for i := 0; i < len(m.Proofs); i++ {

		for j := 0; j < len(proofs); j++ {

			if proofs[j].Secret == m.Proofs[i].Secret {
				m.Proofs[i].State = state
			}
		}
	}

	return nil
}

func (m *MockDB) GetRestoreSigsFromBlindedMessages(tx pgx.Tx, B_ []string) ([]cashu.RecoverSigDB, error) {
	var restore []cashu.RecoverSigDB
	for _, blindMessage := range B_ {
		for _, record := range m.RecoverSigDB {
			B_Hex := hex.EncodeToString(record.B_.SerializeCompressed())
			if blindMessage == B_Hex {
				restore = append(restore, record)
			}
		}
	}
	return restore, nil
}

func (m *MockDB) SaveRestoreSigs(tx pgx.Tx, recover_sigs []cashu.RecoverSigDB) error {
	m.RecoverSigDB = append(m.RecoverSigDB, recover_sigs...)
	return nil

}


func (m *MockDB) GetProofsTimeSeries(since int64, bucketMinutes int) ([]database.ProofTimeSeriesPoint, error) {
	bucketSeconds := int64(bucketMinutes * 60)

	// Determine upper bound
	upperBound := time.Now().Unix()

	// Group proofs by time bucket
	buckets := make(map[int64]*database.ProofTimeSeriesPoint)

	for _, p := range m.Proofs {
		if p.SeenAt < since || p.SeenAt >= upperBound {
			continue
		}

		// Calculate bucket timestamp using floor division
		bucketTimestamp := (p.SeenAt / bucketSeconds) * bucketSeconds

		if _, exists := buckets[bucketTimestamp]; !exists {
			buckets[bucketTimestamp] = &database.ProofTimeSeriesPoint{
				Timestamp:   bucketTimestamp,
				TotalAmount: 0,
				Count:       0,
			}
		}

		buckets[bucketTimestamp].TotalAmount += p.Amount
		buckets[bucketTimestamp].Count++
	}

	// Convert map to sorted slice
	var points []database.ProofTimeSeriesPoint
	for _, point := range buckets {
		points = append(points, *point)
	}

	// Sort by timestamp
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})

	return points, nil
}

func (m *MockDB) GetBlindSigsTimeSeries(since int64, bucketMinutes int) ([]database.ProofTimeSeriesPoint, error) {
	bucketSeconds := int64(bucketMinutes * 60)

	// Determine upper bound
	upperBound := time.Now().Unix()

	// Group blind sigs by time bucket
	buckets := make(map[int64]*database.ProofTimeSeriesPoint)

	for _, sig := range m.RecoverSigDB {
		if sig.CreatedAt < since || sig.CreatedAt >= upperBound {
			continue
		}

		// Calculate bucket timestamp using floor division
		bucketTimestamp := (sig.CreatedAt / bucketSeconds) * bucketSeconds

		if _, exists := buckets[bucketTimestamp]; !exists {
			buckets[bucketTimestamp] = &database.ProofTimeSeriesPoint{
				Timestamp:   bucketTimestamp,
				TotalAmount: 0,
				Count:       0,
			}
		}

		buckets[bucketTimestamp].TotalAmount += sig.Amount
		buckets[bucketTimestamp].Count++
	}

	// Convert map to sorted slice
	var points []database.ProofTimeSeriesPoint
	for _, point := range buckets {
		points = append(points, *point)
	}

	// Sort by timestamp
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})

	return points, nil
}

func (m *MockDB) GetProofsCountByKeyset(since time.Time) (map[string]database.ProofsCountByKeyset, error) {
	results := make(map[string]database.ProofsCountByKeyset)

	for _, p := range m.Proofs {
		if p.SeenAt < since.Unix() {
			continue
		}

		item, exists := results[p.Id]
		if !exists {
			item = database.ProofsCountByKeyset{
				KeysetId:    p.Id,
				TotalAmount: 0,
				Count:       0,
			}
		}

		item.TotalAmount += p.Amount
		item.Count++
		results[p.Id] = item
	}

	return results, nil
}

func (m *MockDB) GetBlindSigsCountByKeyset(since time.Time) (map[string]database.BlindSigsCountByKeyset, error) {
	results := make(map[string]database.BlindSigsCountByKeyset)

	for _, sig := range m.RecoverSigDB {
		if sig.CreatedAt < since.Unix() {
			continue
		}

		item, exists := results[sig.Id]
		if !exists {
			item = database.BlindSigsCountByKeyset{
				KeysetId:    sig.Id,
				TotalAmount: 0,
				Count:       0,
			}
		}

		item.TotalAmount += sig.Amount
		item.Count++
		results[sig.Id] = item
	}

	return results, nil
}
