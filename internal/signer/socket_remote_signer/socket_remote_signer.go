package socketremotesigner

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
	"github.com/lescuer97/nutmix/internal/signer"
	"google.golang.org/grpc"
)

type SocketSigner struct {
	grpcClient sig.SignerClient
}

const abstractSocket = "unix:@signer_socket"

func SetupSocketSigner() (SocketSigner, error) {
	socketSigner := SocketSigner{}

	certs, err := GetTlsSecurityCredential()
	if err != nil {
		return socketSigner, fmt.Errorf("GetTlsSecurityCredential(). %w", err)
	}
	conn, err := grpc.NewClient(abstractSocket,
		grpc.WithTransportCredentials(certs))

	if err != nil {
		log.Fatalf("grpc connection failed: %v", err)
	}

	client := sig.NewSignerClient(conn)

	socketSigner.grpcClient = client
	return socketSigner, nil
}

// gets all active keys
func (s *SocketSigner) GetActiveKeys() (signer.GetKeysResponse, error) {

	ctx := context.Background()
	emptyRequest := sig.EmptyRequest{}
	keys, err := s.grpcClient.ActiveKeys(ctx, &emptyRequest)
	if err != nil {
		return signer.GetKeysResponse{}, fmt.Errorf("s.grpcClient.Pubkey(ctx, &emptyRequest). %w", err)
	}

	return ConvertSigKeysToKeysResponse(keys), nil
}

func (s *SocketSigner) GetKeysById(id string) (signer.GetKeysResponse, error) {

	ctx := context.Background()
	requestId := sig.Id{Id: id}
	keys, err := s.grpcClient.KeysById(ctx, &requestId)
	if err != nil {
		return signer.GetKeysResponse{}, fmt.Errorf("s.grpcClient.KeysById(ctx, &requestId). %w", err)
	}

	return ConvertSigKeysToKeysResponse(keys), nil
}

// gets all keys from the signer
func (s *SocketSigner) GetKeysets() (signer.GetKeysetsResponse, error) {
	ctx := context.Background()
	emptyRequest := sig.EmptyRequest{}
	keys, err := s.grpcClient.Keysets(ctx, &emptyRequest)
	if err != nil {
		return signer.GetKeysetsResponse{}, fmt.Errorf("s.grpcClient.Keysets(ctx, &emptyRequest). %w", err)
	}

	return ConvertSigKeysetsToKeysResponse(keys), nil
}

func (s *SocketSigner) RotateKeyset(unit cashu.Unit, fee uint) error {

	ctx := context.Background()
	rotationReq := sig.RotationRequest{
		Unit: unit.String(),
		Fee:  uint32(fee),
	}
	success, err := s.grpcClient.RotateKeyset(ctx, &rotationReq)
	if err != nil {
		return fmt.Errorf("s.grpcClient.BlindSign(ctx, &blindedMessageRequest). %w", err)
	}

	if !success.Success {
		return fmt.Errorf("Unsuccessful Rotation. %w", cashu.ErrInvalidProof)
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

		blindedMessageRequest.BlindedMessages = append(blindedMessageRequest.BlindedMessages, &sig.BlindedMessage{
			Amount:        val.Amount,
			KeysetId:      val.Id,
			BlindedSecret: B_,
			// Witness: &sig.Witness{} val.Witness,
		})
	}

	blindSigsGrpc, err := s.grpcClient.BlindSign(ctx, &blindedMessageRequest)
	if err != nil {
		return []cashu.BlindSignature{}, []cashu.RecoverSigDB{}, fmt.Errorf("s.grpcClient.BlindSign(ctx, &blindedMessageRequest). %w", err)
	}

	blindSigs := ConvertSigBlindSignaturesToCashuBlindSigs(blindSigsGrpc)
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

func (s *SocketSigner) VerifyProofs(proofs []cashu.Proof, blindMessages []cashu.BlindedMessage) error {

	ctx := context.Background()
	proofsVericationRequest := sig.Proofs{}

	proofsVericationRequest.Proof = make([]*sig.Proof, len(proofs))
	for i, val := range proofs {
		C, err := hex.DecodeString(val.C)
		if err != nil {
			return fmt.Errorf("hex.DecodeString(val.C). %w", err)
		}
		checkOutputs := false
		isProofLocked, spendCondition, witness, err := val.IsProofSpendConditioned(&checkOutputs)

		if err != nil {
			return fmt.Errorf("proof.IsProofSpendConditioned(): %w %w", err, cashu.ErrInvalidProof)
		}

		var sigWitness *sig.Witness = nil

		if isProofLocked {
			sigWitness = ConvertWitnessToGrpc(spendCondition, witness)
		}

		proofsVericationRequest.Proof[i] = &sig.Proof{
			Amount:   val.Amount,
			KeysetId: val.Id,
			C:        C,
			Secret:   []byte(val.Secret),
			Witness:  sigWitness,
		}
	}

	success, err := s.grpcClient.VerifyProofs(ctx, &proofsVericationRequest)
	if err != nil {
		return fmt.Errorf("s.grpcClient.VerifyProofs(ctx, &proofsVericationRequest). %w", err)
	}

	if !success.Success {
		return fmt.Errorf("Invalid proofs. %w", cashu.ErrInvalidProof)
	}
	return nil
}

func (s *SocketSigner) GetSignerPubkey() (string, error) {
	ctx := context.Background()
	emptyRequest := sig.EmptyRequest{}
	pubkey, err := s.grpcClient.Pubkey(ctx, &emptyRequest)

	if err != nil {
		return "", fmt.Errorf("s.grpcClient.Pubkey(ctx, &emptyRequest). %w", err)
	}

	return hex.EncodeToString(pubkey.Pubkey), nil
}
