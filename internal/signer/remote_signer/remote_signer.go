package remotesigner

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
	"github.com/lescuer97/nutmix/internal/signer"
	"google.golang.org/grpc"
)

type MintPublicKeyset struct {
	Id          []byte
	Unit        string
	Active      bool
	Keys        map[uint64]string
	InputFeePpk uint
}

type SocketSigner struct {
	grpcClient    sig.SignerServiceClient
	activeKeysets map[string]MintPublicKeyset
	keysets       map[string]MintPublicKeyset
	pubkey        []byte
}

const abstractSocket = "unix:@signer_socket"

func SetupRemoteSigner(connectToNetwork bool, networkAddress string) (SocketSigner, error) {
	socketSigner := SocketSigner{}

	certs, err := GetTlsSecurityCredential()
	if err != nil {
		return socketSigner, fmt.Errorf("GetTlsSecurityCredential(). %w", err)
	}


	target := abstractSocket
	if connectToNetwork {
		target = networkAddress
	}

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(certs))

	if err != nil {
		log.Fatalf("grpc connection failed: %v", err)
	}

	client := sig.NewSignerServiceClient(conn)

	socketSigner.grpcClient = client
	socketSigner.keysets = make(map[string]MintPublicKeyset)
	socketSigner.activeKeysets = make(map[string]MintPublicKeyset)

	err = socketSigner.setupSignerPubkeys()
	if err != nil {
		log.Fatalf("socketSigner.setupSignerPubkeys(): %v", err)
	}

	return socketSigner, nil
}

// gets all active keys
func (s *SocketSigner) setupSignerPubkeys() error {

	ctx := context.Background()
	emptyRequest := sig.EmptyRequest{}
	keys, err := s.grpcClient.Keysets(ctx, &emptyRequest)
	if err != nil {
		return fmt.Errorf("s.grpcClient.Pubkey(ctx, &emptyRequest). %w", err)
	}

	err = CheckIfSignerErrorExists(keys.GetError())
	if err != nil {
		return fmt.Errorf("CheckIfSignerErrorExists(keys.GetError()). %w", err)
	}

	if keys.GetKeysets() == nil {
		return fmt.Errorf("No keysets on the signer. %w", err)
	}

	s.pubkey = keys.GetKeysets().Pubkey

	for i, key := range keys.GetKeysets().Keysets {
		if key == nil {
			return fmt.Errorf("There was a nil key. index: %v", i)
		}

		if key.Keys == nil {
			return fmt.Errorf("No keys on keyset. Id: %v", key.Id)
		}

		unit, err := ConvertSigUnitToCashuUnit(key.Unit)
		if err != nil {
			return fmt.Errorf("ConvertSigUnitToCashuUnit(key.Unit). %w", err)
		}
		mintKeyset := MintPublicKeyset{
			Id:          key.Id,
			Unit:        unit.String(),
			Active:      key.Active,
			InputFeePpk: uint(key.InputFeePpk),
		}

		stringKeys := make(map[uint64]string)

		for key, val := range key.GetKeys().GetKeys() {
			stringKeys[key] = hex.EncodeToString(val)
		}
		mintKeyset.Keys = stringKeys

		if mintKeyset.Active {
			s.activeKeysets[hex.EncodeToString(mintKeyset.Id)] = mintKeyset
		}

		s.keysets[hex.EncodeToString(mintKeyset.Id)] = mintKeyset
	}
	return nil
}

// gets all active keys
func (s *SocketSigner) GetActiveKeys() (signer.GetKeysResponse, error) {
	var keys []MintPublicKeyset
	for _, keyset := range s.activeKeysets {
		keys = append(keys, keyset)
	}
	return OrderKeysetByUnit(keys), nil
}

func (s *SocketSigner) GetKeysById(id string) (signer.GetKeysResponse, error) {
	val, exists := s.keysets[id]
	if exists {
		return OrderKeysetByUnit([]MintPublicKeyset{val}), nil
	}
	return signer.GetKeysResponse{}, cashu.ErrKeysetNotFound
}

// gets all keys from the signer
func (s *SocketSigner) GetKeysets() (signer.GetKeysetsResponse, error) {

	var response signer.GetKeysetsResponse
	for _, seed := range s.keysets {
		response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{Id: hex.EncodeToString(seed.Id), Unit: seed.Unit, Active: seed.Active, InputFeePpk: seed.InputFeePpk})
	}
	return response, nil
}

func (s *SocketSigner) RotateKeyset(unit cashu.Unit, fee uint) error {

	ctx := context.Background()

	unitSig, err := ConvertCashuUnitToSignature(unit)
	if err != nil {
		return fmt.Errorf("ConvertCashuUnitToSignature(unit). %w", err)
	}

	amounts := GetAmountsFromMaxOrder(32)
	rotationReq := sig.RotationRequest{
		Unit:        unitSig,
		InputFeePpk: uint64(fee),
		Amounts:     amounts,
	}
	rotationResponse, err := s.grpcClient.RotateKeyset(ctx, &rotationReq)
	if err != nil {
		return fmt.Errorf("s.grpcClient.BlindSign(ctx, &blindedMessageRequest). %w", err)
	}
	err = CheckIfSignerErrorExists(rotationResponse.GetError())
	if err != nil {
		return fmt.Errorf("CheckIfSignerErrorExists(rotationResponse.GetError()). %w", err)
	}

	err = s.setupSignerPubkeys()
	if err != nil {
		return fmt.Errorf("s.setupSignerPubkeys(). %w", err)
	}

	return nil
}

func (s *SocketSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {

	ctx := context.Background()
	blindedMessageRequest := sig.BlindedMessages{}

	blindedMessageRequest.BlindedMessages = []*sig.BlindedMessage{}
	for _, val := range messages {
		B_, err := hex.DecodeString(val.B_)
		if err != nil {
			return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("hex.DecodeString(val.B_). %w", err)
		}
		bytesId, err := hex.DecodeString(val.Id)
		if err != nil {
			return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("hex.DecodeString(val.Id). %w", err)
		}

		blindedMessageRequest.BlindedMessages = append(blindedMessageRequest.BlindedMessages, &sig.BlindedMessage{
			Amount:        val.Amount,
			KeysetId:      bytesId,
			BlindedSecret: B_,
			// Witness: &sig.Witness{} val.Witness,
		})
	}

	blindSigsResponse, err := s.grpcClient.BlindSign(ctx, &blindedMessageRequest)
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("s.grpcClient.BlindSign(ctx, &blindedMessageRequest). %w", err)
	}
	err = CheckIfSignerErrorExists(blindSigsResponse.GetError())
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("CheckIfSignerErrorExists(blindSigsResponse.GetError()). %w", err)
	}

	blindSigs := ConvertSigBlindSignaturesToCashuBlindSigs(blindSigsResponse)
	// verify we have the same amount of blindedmessages than BlindSignatures
	if len(blindSigs) != len(messages) {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("Not the correct amount of blind signatures")
	}

	recoverySigs := []cashu.RecoverSigDB{}
	now := time.Now().Unix()
	// make recovery signatures
	for i, val := range blindSigs {
		recoverySigs = append(recoverySigs, cashu.RecoverSigDB{
			Amount:    val.Amount,
			Id:        val.Id,
			C_:        val.C_,
			B_:        messages[i].B_,
			CreatedAt: now,
			Dleq:      val.Dleq,
		})

	}

	return blindSigs, recoverySigs, nil
}
func (l *SocketSigner) validateIfLockedProof(proof cashu.Proof, checkOutputs *bool, pubkeysFromProofs *map[*btcec.PublicKey]bool) error {

	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, witness, err := proof.IsProofSpendConditioned(checkOutputs)

	if err != nil {
		return fmt.Errorf("proof.IsProofSpendConditioned(): %w %w", err, cashu.ErrInvalidProof)
	}

	if isProofLocked {
		ok, err := proof.VerifyWitness(spendCondition, witness, pubkeysFromProofs)

		if err != nil {
			return fmt.Errorf("proof.VerifyWitnessSig(): %w", err)
		}

		if !ok {
			return cashu.ErrInvalidProof
		}
	}
	return nil
}

func (s *SocketSigner) VerifyProofs(proofs []cashu.Proof, blindMessages []cashu.BlindedMessage) error {

	ctx := context.Background()
	// INFO: we verify locally if the proofs are locked and valid before sending to the crypto signer
	proofsVericationRequest := sig.Proofs{}

	proofsVericationRequest.Proof = make([]*sig.Proof, len(proofs))
	pubkeysFromProofs := make(map[*btcec.PublicKey]bool)
	verifyOutputs := false
	for i, val := range proofs {
		err := s.validateIfLockedProof(val, &verifyOutputs, &pubkeysFromProofs)
		if err != nil {
			return fmt.Errorf("s.validateIfLockedProof(val, &verifyOutputs, &pubkeysFromProofs). %w", err)
		}

		C, err := hex.DecodeString(val.C)
		if err != nil {
			return fmt.Errorf("hex.DecodeString(val.C). %w", err)
		}

		if err != nil {
			return fmt.Errorf("proof.IsProofSpendConditioned(): %w %w", err, cashu.ErrInvalidProof)
		}

		bytesId, err := hex.DecodeString(val.Id)
		if err != nil {
			return fmt.Errorf("hex.DecodeString(val.Id). %w", err)
		}

		proofsVericationRequest.Proof[i] = &sig.Proof{
			Amount:   val.Amount,
			KeysetId: bytesId,
			C:        C,
			Secret:   []byte(val.Secret),
		}
	}

	boolResponse, err := s.grpcClient.VerifyProofs(ctx, &proofsVericationRequest)
	if err != nil {
		return fmt.Errorf("s.grpcClient.VerifyProofs(ctx, &proofsVericationRequest). %w", err)
	}

	err = CheckIfSignerErrorExists(boolResponse.GetError())
	if err != nil {
		return fmt.Errorf("CheckIfSignerErrorExists(boolResponse.GetError()). %w", err)
	}

	if !boolResponse.GetSuccess() {
		return fmt.Errorf("Invalid proofs. %w", cashu.ErrInvalidProof)
	}
	return nil
}

func (s *SocketSigner) GetSignerPubkey() (string, error) {

	return hex.EncodeToString(s.pubkey), nil
}

// gets all active keys
func (l *SocketSigner) GetAuthActiveKeys() (signer.GetKeysResponse, error) {
	var keys []MintPublicKeyset
	for _, keyset := range l.activeKeysets {
		if keyset.Unit == cashu.AUTH.String() {
			keys = append(keys, keyset)
		}
	}

	if len(keys) == 0 {
		return signer.GetKeysResponse{}, cashu.ErrKeysetNotFound
	}

	return OrderKeysetByUnit(keys), nil
}

func (s *SocketSigner) GetAuthKeysById(id string) (signer.GetKeysResponse, error) {

	val, exists := s.keysets[id]
	if exists {
		if val.Unit == cashu.AUTH.String() {
			return OrderKeysetByUnit([]MintPublicKeyset{val}), nil
		}
	}
	return signer.GetKeysResponse{}, cashu.ErrKeysetNotFound
}

// gets all keys from the signer
func (l *SocketSigner) GetAuthKeys() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	for _, key := range l.keysets {
		if key.Unit == cashu.AUTH.String() {
			response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{Id: hex.EncodeToString(key.Id), Unit: key.Unit, Active: key.Active, InputFeePpk: key.InputFeePpk})
		}
	}
	return response, nil
}

func GetAmountsFromMaxOrder(max_order uint32) []uint64 {
	keys := make([]uint64, 0)

	for i := 0; i < int(max_order); i++ {
		keys = append(keys, uint64(math.Pow(2, float64(i))))
	}
	return keys
}
