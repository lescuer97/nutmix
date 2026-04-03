package remotesigner

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
	"github.com/lescuer97/nutmix/internal/signer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type MintPublicKeyset struct {
	Keys          map[uint64]string
	FinalExpiry   *uint64
	IssuerVersion *string
	Unit          string
	Id            []byte
	InputFeePpk   uint
	Version       uint32
	Active        bool
}

type RemoteSigner struct {
	grpcClient    sig.SignatoryClient
	activeKeysets map[string]MintPublicKeyset
	keysets       map[string]MintPublicKeyset
	pubkey        []byte
}

const abstractSocket = "unix:@signer_socket"

func SetupRemoteSigner(connectToNetwork bool, networkAddress string) (RemoteSigner, error) {
	socketSigner := RemoteSigner{
		grpcClient:    nil,
		activeKeysets: make(map[string]MintPublicKeyset),
		keysets:       make(map[string]MintPublicKeyset),
		pubkey:        nil,
	}

	certs, err := GetTlsSecurityCredential()
	if err != nil {
		return socketSigner, fmt.Errorf("GetTlsSecurityCredential(). %w", err)
	}

	target := abstractSocket
	if connectToNetwork {
		target = networkAddress
	}

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(certs),
		grpc.WithUnaryInterceptor(clientVersionInterceptor()),
	)

	if err != nil {
		log.Fatalf("grpc connection failed: %v", err)
	}

	client := sig.NewSignatoryClient(conn)

	socketSigner.grpcClient = client

	err = socketSigner.setupSignerPubkeys()
	if err != nil {
		log.Fatalf("socketSigner.setupSignerPubkeys(): %v", err)
	}

	return socketSigner, nil
}

func clientVersionInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-signatory-schema-version", strconv.FormatUint(uint64(sig.Constants_CONSTANTS_VERSION), 10))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// gets all active keys
func (s *RemoteSigner) setupSignerPubkeys() error {
	ctx := context.Background()
	emptyRequest := sig.EmptyRequest{}

	keys, err := s.grpcClient.Keysets(ctx, &emptyRequest)
	if err != nil {
		return fmt.Errorf("s.grpcClient.Keysets(ctx, &emptyRequest). %w", err)
	}
	if err := signerValidator.Struct(keys); err != nil {
		return fmt.Errorf("signer keysets response validation failed: %w", err)
	}

	err = CheckIfSignerErrorExists(keys.GetError())
	if err != nil {
		return fmt.Errorf("CheckIfSignerErrorExists(keys.GetError()). %w", err)
	}

	if keys.GetKeysets() == nil {
		return fmt.Errorf("no keysets on the signer: %w", err)
	}

	if err := signerValidator.Struct(keys.GetKeysets()); err != nil {
		return fmt.Errorf("signer keysets payload validation failed: %w", err)
	}

	s.pubkey = keys.GetKeysets().Pubkey

	for i, key := range keys.GetKeysets().Keysets {
		if key == nil {
			return fmt.Errorf("there was a nil key, index: %v", i)
		}
		if err := signerValidator.Struct(key); err != nil {
			return fmt.Errorf("signer keyset validation failed at index %d: %w", i, err)
		}

		if key.Keys == nil {
			return fmt.Errorf("no keys on keyset, id: %v", key.Id)
		}
		if err := signerValidator.Struct(key.Keys); err != nil {
			return fmt.Errorf("signer keyset keys validation failed at index %d: %w", i, err)
		}
		if key.Unit == nil {
			return fmt.Errorf("signer keyset missing unit at index %d", i)
		}

		unit, err := ConvertSigUnitToCashuUnit(key.Unit)
		if err != nil {
			return fmt.Errorf("ConvertSigUnitToCashuUnit(key.Unit). %w", err)
		}
		stringKeys := make(map[uint64]string)

		for key, val := range key.GetKeys().GetKeys() {
			stringKeys[key] = hex.EncodeToString(val)
		}

		mintKeyset := MintPublicKeyset{
			Id:            key.Id,
			Unit:          unit.String(),
			Active:        key.Active,
			InputFeePpk:   uint(key.InputFeePpk),
			Version:       key.Version,
			FinalExpiry:   key.FinalExpiry,
			Keys:          stringKeys,
			IssuerVersion: key.IssuerVersion,
		}

		if mintKeyset.Active {
			s.activeKeysets[hex.EncodeToString(mintKeyset.Id)] = mintKeyset
		}

		s.keysets[hex.EncodeToString(mintKeyset.Id)] = mintKeyset
	}

	return nil
}

// gets all active keys
func (s *RemoteSigner) GetActiveKeys() (signer.GetKeysResponse, error) {
	keys := make([]MintPublicKeyset, len(s.activeKeysets))
	indexActiveKeysets := 0
	for _, keyset := range s.activeKeysets {
		keys[indexActiveKeysets] = keyset
		indexActiveKeysets++
	}
	return OrderKeysetByUnit(keys), nil
}

func (s *RemoteSigner) GetKeysById(id string) (signer.GetKeysResponse, error) {
	val, exists := s.keysets[id]
	if exists {
		return OrderKeysetByUnit([]MintPublicKeyset{val}), nil
	}
	return signer.GetKeysResponse{}, cashu.ErrKeysetNotFound
}

// gets all keys from the signer
func (s *RemoteSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	for _, seed := range s.keysets {
		if seed.Unit != cashu.AUTH.String() {
			response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{
				Id:          hex.EncodeToString(seed.Id),
				Unit:        seed.Unit,
				Active:      seed.Active,
				InputFeePpk: seed.InputFeePpk,
				Version:     seed.Version,
				FinalExpiry: seed.FinalExpiry,
			})
		}
	}
	return response, nil
}

func (s *RemoteSigner) RotateKeyset(unit cashu.Unit, fee uint, expiry_limit_hours uint) error {
	ctx := context.Background()

	unitSig, err := ConvertCashuUnitToSignature(unit)
	if err != nil {
		return fmt.Errorf("ConvertCashuUnitToSignature(unit). %w", err)
	}

	now := time.Now()
	now = now.Add(time.Duration(expiry_limit_hours) * time.Hour)

	amounts := cashu.GetAmountsForKeysets(cashu.MaxKeysetAmount)
	if unit == cashu.AUTH {
		amounts = []uint64{amounts[0]}
	}

	unixTime := uint64(now.Unix())
	rotationReq := sig.RotationRequest{
		Unit:         unitSig,
		InputFeePpk:  uint64(fee),
		Amounts:      amounts,
		FinalExpiry:  &unixTime,
		KeysetIdType: sig.KeysetVersion_KEYSET_VERSION_V2,
	}
	rotationResponse, err := s.grpcClient.RotateKeyset(ctx, &rotationReq)
	if err != nil {
		return fmt.Errorf("s.grpcClient.RotateKeyset(ctx, &rotationReq). %w", err)
	}
	if err := signerValidator.Struct(rotationResponse); err != nil {
		return fmt.Errorf("signer rotate keyset response validation failed: %w", err)
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

func (s *RemoteSigner) SignBlindMessages(messages []cashu.BlindedMessage) ([]cashu.BlindSignature, []cashu.RecoverSigDB, error) {
	ctx := context.Background()
	blindedMessageRequest := sig.BlindedMessages{
		BlindedMessages: make([]*sig.BlindedMessage, len(messages)),
	}

	for i := range messages {
		B_ := messages[i].B_.SerializeCompressed()

		bytesId, err := hex.DecodeString(messages[i].Id)
		if err != nil {
			return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("hex.DecodeString(val.Id). %w", err)
		}

		blindedMessageRequest.BlindedMessages[i] = &sig.BlindedMessage{
			Amount:        messages[i].Amount,
			KeysetId:      bytesId,
			BlindedSecret: B_,
		}
	}

	blindSigsResponse, err := s.grpcClient.BlindSign(ctx, &blindedMessageRequest)
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("s.grpcClient.BlindSign(ctx, &blindedMessageRequest). %w", err)
	}
	err = CheckIfSignerErrorExists(blindSigsResponse.GetError())
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("CheckIfSignerErrorExists(blindSigsResponse.GetError()). %w", err)
	}

	blindSigs, err := ConvertSigBlindSignaturesToCashuBlindSigs(blindSigsResponse)
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("CheckIfSignerErrorExists(blindSigsResponse.GetError()). %w", err)
	}
	// verify we have the same amount of blindedmessages than BlindSignatures
	if len(blindSigs) != len(messages) {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("not the correct amount of blind signatures")
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
			MeltQuote: "",
		})
	}

	return blindSigs, recoverySigs, nil
}

func (s *RemoteSigner) VerifyProofs(proofs []cashu.Proof) error {
	ctx := context.Background()
	// INFO: we verify locally if the proofs are locked and valid before sending to the crypto signer
	proofsVericationRequest := sig.Proofs{
		Proof: make([]*sig.Proof, len(proofs)),
	}
	for i, val := range proofs {
		C := val.C.SerializeCompressed()

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
	if err := signerValidator.Struct(boolResponse); err != nil {
		return fmt.Errorf("signer verify proofs response validation failed: %w", err)
	}

	err = CheckIfSignerErrorExists(boolResponse.GetError())
	if err != nil {
		return fmt.Errorf("CheckIfSignerErrorExists(boolResponse.GetError()). %w", err)
	}

	if !boolResponse.GetSuccess() {
		return fmt.Errorf("invalid proofs: %w", cashu.ErrInvalidProof)
	}
	return nil
}

func (s *RemoteSigner) GetSignerPubkey() (string, error) {
	return hex.EncodeToString(s.pubkey), nil
}

// gets all active keys
func (l *RemoteSigner) GetAuthActiveKeys() (signer.GetKeysResponse, error) {
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

func (s *RemoteSigner) GetAuthKeysById(id string) (signer.GetKeysResponse, error) {
	val, exists := s.keysets[id]
	if exists {
		if val.Unit == cashu.AUTH.String() {
			return OrderKeysetByUnit([]MintPublicKeyset{val}), nil
		}
	}
	return signer.GetKeysResponse{}, cashu.ErrKeysetNotFound
}

// gets all keys from the signer
func (l *RemoteSigner) GetAuthKeys() (signer.GetKeysetsResponse, error) {
	var response signer.GetKeysetsResponse
	for _, key := range l.keysets {
		if key.Unit == cashu.AUTH.String() {
			response.Keysets = append(response.Keysets, cashu.BasicKeysetResponse{
				Id:          hex.EncodeToString(key.Id),
				Unit:        key.Unit,
				Active:      key.Active,
				InputFeePpk: key.InputFeePpk,
				Version:     key.Version,
				FinalExpiry: key.FinalExpiry,
			})
		}
	}
	return response, nil
}

// func GetAmountsFromMaxOrder(max_order uint32) []uint64 {
// 	keys := make([]uint64, 0)
//
// 	for i := 0; i < int(max_order); i++ {
// 		keys = append(keys, uint64(math.Pow(2, float64(i))))
// 	}
// 	return keys
// }
