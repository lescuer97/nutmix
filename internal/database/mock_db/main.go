package mockdb

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/utils"
)

var ErrDB = errors.New("ERROR DATABASE")

var DATABASE_URL_ENV = "DATABASE_URL"

type MockDB struct {
	GetConfigErr                     error
	GetSeedsByUnitErr                error
	UpdateNostrNotificationConfigErr error
	NostrNotificationConfig          *utils.NostrNotificationConfig
	LastLightningSearch              *string
	MeltChange                       []cashu.MeltChange
	Proofs                           []cashu.Proof
	Stats                            []database.StatsSnapshot
	RecoverSigDB                     []cashu.RecoverSigDB
	NostrAuth                        []database.NostrLoginAuth
	LiquiditySwap                    []utils.LiquiditySwap
	MeltRequest                      []cashu.MeltRequestDB
	Seeds                            []cashu.Seed
	AuthUser                         []database.AuthUser
	MintRequest                      []cashu.MintRequestDB
	Config                           utils.Config
	LastStatsSince                   int64
	LastMintSince                    int64
	LastMeltSince                    int64
	ReturnError                      int64
	LastSearchSince                  int64
	LastSearchLimit                  int
}

func databaseError(err error) error {
	return errors.Join(ErrDB, err)
}

func (m *MockDB) GetAllSeeds() ([]cashu.Seed, error) {
	return m.Seeds, nil
}
func (m *MockDB) GetTx(ctx context.Context) (pgx.Tx, error) {
	return &pgxpool.Tx{}, nil
}
func (m *MockDB) Commit(ctx context.Context, tx pgx.Tx) error {
	return nil
}
func (m *MockDB) Rollback(ctx context.Context, tx pgx.Tx) error {
	return nil
}

func (m *MockDB) GetSeedsByUnit(tx pgx.Tx, unit cashu.Unit) ([]cashu.Seed, error) {
	if m.GetSeedsByUnitErr != nil {
		return nil, m.GetSeedsByUnitErr
	}
	seeds := []cashu.Seed{}
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

func (m *MockDB) ChangeMintRequestState(tx pgx.Tx, quote string, state cashu.ACTION_STATE, minted bool) error {
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

func (m *MockDB) GetMeltRequestById(ctx context.Context, tx pgx.Tx, id string) (cashu.MeltRequestDB, error) {
	if err := ctx.Err(); err != nil {
		return cashu.MeltRequestDB{}, err
	}
	var meltRequests []cashu.MeltRequestDB
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == id {
			meltRequests = append(meltRequests, m.MeltRequest[i])
		}
	}
	if len(meltRequests) == 0 {
		return cashu.MeltRequestDB{}, pgx.ErrNoRows
	}

	quote := meltRequests[0]
	quote.Change = nil
	for _, sig := range m.RecoverSigDB {
		if sig.MeltQuote == id {
			quote.Change = append(quote.Change, sig.GetBlindSignature())
		}
	}
	return quote, nil
}
func (m *MockDB) ListMeltQuotesForReconciliation(ctx context.Context) ([]cashu.MeltRequestDB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var meltRequests []cashu.MeltRequestDB
	for i := range m.MeltRequest {
		quote := m.MeltRequest[i]
		if quote.State == cashu.PENDING {
			meltRequests = append(meltRequests, quote)
			continue
		}
		if quote.State != cashu.PAID || !quote.Melted {
			continue
		}
		for _, proof := range m.Proofs {
			if proof.Quote != nil && *proof.Quote == quote.Quote && proof.State == cashu.PROOF_PENDING {
				meltRequests = append(meltRequests, quote)
				break
			}
		}
	}
	return meltRequests, nil
}

func (m *MockDB) SaveMeltRequest(tx pgx.Tx, request cashu.MeltRequestDB) error {
	m.MeltRequest = append(m.MeltRequest, request)

	return nil
}

func (m *MockDB) AddPreimageMeltRequest(ctx context.Context, tx pgx.Tx, quote string, preimage string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].PaymentPreimage = preimage
		}
	}
	return nil
}
func (m *MockDB) ChangeMeltRequestState(ctx context.Context, tx pgx.Tx, quote string, state cashu.ACTION_STATE, melted bool, paid_fee uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].State = state
			m.MeltRequest[i].Melted = melted
			m.MeltRequest[i].FeePaid = paid_fee
		}
	}
	return nil
}
func (m *MockDB) ChangeCheckingId(ctx context.Context, tx pgx.Tx, quote string, checking_id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(m.MeltRequest); i++ {
		if m.MeltRequest[i].Quote == quote {
			m.MeltRequest[i].CheckingId = checking_id
		}
	}
	return nil
}

func (m *MockDB) GetProofsFromSecret(ctx context.Context, tx pgx.Tx, SecretList []string) (cashu.Proofs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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
func (m *MockDB) GetProofsFromQuote(ctx context.Context, tx pgx.Tx, quote string) (cashu.Proofs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

func (m *MockDB) SaveProof(ctx context.Context, tx pgx.Tx, proofs []cashu.Proof) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.Proofs = append(m.Proofs, proofs...)
	return nil
}

func (m *MockDB) GetProofsFromSecretCurve(ctx context.Context, tx pgx.Tx, Ys []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
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

func (m *MockDB) DeleteProofs(ctx context.Context, tx pgx.Tx, proofs cashu.Proofs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(m.Proofs); i++ {
		for j := 0; j < len(proofs); j++ {
			if proofs[j].Y == m.Proofs[i].Y {
				m.Proofs = append(m.Proofs[:i], m.Proofs[i+1:]...)
			}
		}
	}

	return nil
}

func (m *MockDB) SetProofsState(ctx context.Context, tx pgx.Tx, proofs cashu.Proofs, state cashu.ProofState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(m.Proofs); i++ {
		for j := 0; j < len(proofs); j++ {
			if proofs[j].Secret == m.Proofs[i].Secret {
				m.Proofs[i].State = state
			}
		}
	}

	return nil
}

func (m *MockDB) GetRestoreSigsFromBlindedMessages(ctx context.Context, tx pgx.Tx, B_ []cashu.WrappedPublicKey) ([]cashu.RecoverSigDB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var restore []cashu.RecoverSigDB
	for _, blindMessage := range B_ {
		for _, record := range m.RecoverSigDB {
			B_Hex := hex.EncodeToString(record.B_.SerializeCompressed())
			if blindMessage.ToHex() == B_Hex {
				restore = append(restore, record)
			}
		}
	}
	return restore, nil
}

func (m *MockDB) InsertSignedOutputs(ctx context.Context, tx pgx.Tx, recover_sigs []cashu.SignedOutput) error {
	return m.insertSignedOutputs(ctx, tx, "", recover_sigs)
}

func (m *MockDB) InsertMintSignedOutputs(ctx context.Context, tx pgx.Tx, quoteID string, recover_sigs []cashu.SignedOutput) error {
	if quoteID == "" {
		return cashu.ErrInvalidBlindMessage
	}
	return m.insertSignedOutputs(ctx, tx, quoteID, recover_sigs)
}

func (m *MockDB) insertSignedOutputs(ctx context.Context, tx pgx.Tx, quoteID string, recover_sigs []cashu.SignedOutput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[string]bool, len(recover_sigs))
	points := make([]cashu.WrappedPublicKey, len(recover_sigs))
	for i, sig := range recover_sigs {
		if sig.B_.PublicKey == nil || sig.C_.PublicKey == nil || sig.Amount == 0 || sig.Id == "" ||
			(sig.Dleq != nil && (sig.Dleq.E == nil || sig.Dleq.S == nil)) {
			return cashu.ErrInvalidBlindMessage
		}
		key := sig.B_.ToHex()
		if seen[key] {
			return cashu.ErrRepeatedOutput
		}
		seen[key] = true
		points[i] = sig.B_
	}
	if err := m.CheckBlindMessagesAvailable(ctx, tx, points); err != nil {
		return err
	}
	for _, sig := range recover_sigs {
		m.RecoverSigDB = append(m.RecoverSigDB, cashu.RecoverSigDB{B_: sig.B_, C_: sig.C_, Dleq: sig.Dleq, Id: sig.Id, Amount: sig.Amount, CreatedAt: sig.CreatedAt, MeltQuote: "", MintQuote: quoteID})
	}
	return nil
}

func (m *MockDB) CheckBlindMessagesAvailable(ctx context.Context, _ pgx.Tx, points []cashu.WrappedPublicKey) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, point := range points {
		if point.PublicKey == nil {
			return cashu.ErrInvalidBlindMessage
		}
		key := point.ToHex()
		for _, stored := range m.RecoverSigDB {
			if stored.B_.ToHex() == key {
				return cashu.ErrBlindMessageAlreadySigned
			}
		}
		for _, pending := range m.MeltChange {
			if pending.B_.ToHex() == key {
				return cashu.ErrOutputPending
			}
		}
	}
	return nil
}
