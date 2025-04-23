package socketremotesigner

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
)

func ConvertSigBlindSignaturesToCashuBlindSigs(sigs *sig.BlindSignResponse) []cashu.BlindSignature {
	blindSigs := []cashu.BlindSignature{}

	if sigs == nil {
		return blindSigs
	}

	blindSigs = []cashu.BlindSignature{}

	for _, val := range sigs.GetSigs().BlindSignatures {
		dleq := cashu.BlindSignatureDLEQ{
			E: secp256k1.PrivKeyFromBytes(val.Dleq.E),
			S: secp256k1.PrivKeyFromBytes(val.Dleq.S),
		}
		blindSigs = append(blindSigs, cashu.BlindSignature{Amount: val.Amount, C_: hex.EncodeToString(val.BlindedSecret), Id: val.KeysetId, Dleq: &dleq})
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

		messagesGrpc.BlindedMessages[i] = &sig.BlindedMessage{
			Amount:        val.Amount,
			KeysetId:      val.Id,
			BlindedSecret: B_,
			// Witness: &sig.Witness{} val.Witness,
		}
	}

	return &messagesGrpc, nil
}

func ConvertCashuUnitToSignature(unit cashu.Unit) (*sig.CurrencyUnit, error) {
	switch unit {
	case cashu.Sat:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_SAT}}, nil
	case cashu.Msat:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_MSAT}}, nil
	case cashu.EUR:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_EUR}}, nil
	case cashu.AUTH:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_AUTH}}, nil

	default:
		return nil, fmt.Errorf("No available unit.")
	}
}

func ConvertSigUnitToCashuUnit(sigUnit *sig.CurrencyUnit) (cashu.Unit, error) {
	switch sigUnit.GetUnit().Number() {
	case sig.CurrencyUnitType_SAT.Enum().Number():
		return cashu.Sat, nil
	case sig.CurrencyUnitType_MSAT.Enum().Number():
		return cashu.Msat, nil
	case sig.CurrencyUnitType_EUR.Enum().Number():
		return cashu.EUR, nil
	case sig.CurrencyUnitType_USD.Enum().Number():
		return cashu.USD, nil
	case sig.CurrencyUnitType_AUTH.Enum().Number():
		return cashu.AUTH, nil

	default:
		unit, err := cashu.UnitFromString(strings.ToLower(sigUnit.GetCustomUnit()))

		if err != nil {
			return cashu.Sat, fmt.Errorf("cashu.UnitFromString(strings.ToLower(req.Unit.String())). %w", err)
		}
		return unit, nil

	}

}

func CheckIfSignerErrorExists(err *sig.Error) error {
	if err == nil {
		return nil
	}

	switch err.Code {
	default:
		return fmt.Errorf("Unknown error happened with the signer")
	}
}

func ConvertWitnessToGrpc(spendCondition *cashu.SpendCondition, witness *cashu.Witness) *sig.Witness {
	if witness == nil {
		return nil
	}

	var sigWitness *sig.Witness = nil
	stringSignatures := []string{}
	for i := range witness.Signatures {
		stringSignatures = append(stringSignatures, hex.EncodeToString(witness.Signatures[i].Serialize()))
	}

	switch spendCondition.Type {
	case cashu.P2PK:
		sigWitness = &sig.Witness{
			WitnessType: nil,
		}
		p2pkWitness := sig.P2PKWitness{
			Signatures: stringSignatures,
		}

		sigWitness.WitnessType = &sig.Witness_P2PkWitness{
			P2PkWitness: &p2pkWitness,
		}
	case cashu.HTLC:
		sigWitness = &sig.Witness{
			WitnessType: nil,
		}
		htlcWitness := sig.HTLCWitness{
			Signatures: stringSignatures,
			Preimage:   witness.Preimage,
		}
		sigWitness.WitnessType = &sig.Witness_HtlcWitness{
			HtlcWitness: &htlcWitness,
		}
	}

	return sigWitness
}
