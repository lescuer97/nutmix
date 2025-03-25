package socketremotesigner

import (
	"encoding/hex"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
	"github.com/lescuer97/nutmix/internal/signer"
)

func ConvertSigKeysToKeysResponse(keys *sig.KeysResponse) signer.GetKeysResponse {
	sigs := signer.GetKeysResponse{}

	if keys == nil {
		return sigs
	}

	sigs.Keysets = make([]signer.KeysetResponse, len(keys.Keysets))
	for i, val := range keys.Keysets {
		sigs.Keysets[i] = signer.KeysetResponse{Id: val.Id, Unit: val.Unit, Keys: val.Keys, InputFeePpk: uint(val.InputFeePpk)}
	}

	return sigs
}

func ConvertSigKeysetsToKeysResponse(keys *sig.KeysetResponse) signer.GetKeysetsResponse {
	keysets := signer.GetKeysetsResponse{}

	if keys == nil {
		keysets.Keysets = make([]cashu.BasicKeysetResponse, 0)
		return keysets
	}

	keysets.Keysets = make([]cashu.BasicKeysetResponse, len(keys.Keysets))

	for i, val := range keys.Keysets {
		keysets.Keysets[i] = cashu.BasicKeysetResponse{Id: val.Id, Unit: val.Unit, Active: val.Active, InputFeePpk: uint(val.InputFeePpk)}
	}

	return keysets
}
func ConvertSigBlindSignaturesToCashuBlindSigs(sigs *sig.BlindSignatures) []cashu.BlindSignature {
	blindSigs := []cashu.BlindSignature{}

	if sigs == nil {
		return blindSigs
	}

	blindSigs = []cashu.BlindSignature{}

	for _, val := range sigs.BlindSignatures {
		dleq := cashu.BlindSignatureDLEQ{
			E: secp256k1.PrivKeyFromBytes(val.Dleq.E),
			S: secp256k1.PrivKeyFromBytes(val.Dleq.S),
		}
		blindSigs = append(blindSigs,  cashu.BlindSignature{Amount: val.Amount, C_: hex.EncodeToString(val.BlindedSecret), Id: val.KeysetId, Dleq: &dleq })
	}

	return blindSigs
}


func ConvertBlindedMessagedToGRPC(messages []cashu.BlindedMessage) (*sig.BlindedMessages, error) {
	messagesGrpc := sig.BlindedMessages{
		BlindedMessages: make([]*sig.BlindedMessage, len(messages)),
	}

	for i, val := range messages {
		B_, err := hex.DecodeString(val.B_)
		if err != nil {
			return &messagesGrpc, fmt.Errorf("hex.DecodeString(val.B_). %w", err)
		}

		messagesGrpc.BlindedMessages[i] =  &sig.BlindedMessage{
			Amount: val.Amount,
			KeysetId: val.Id,
			BlindedSecret: B_,
			// Witness: &sig.Witness{} val.Witness,
		}
	}

	return &messagesGrpc, nil
}
