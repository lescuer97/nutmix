package remotesigner

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	sig "github.com/lescuer97/nutmix/internal/gen"
)

func ConvertSigBlindSignaturesToCashuBlindSigs(sigs *sig.BlindSignResponse) ([]cashu.BlindSignature, error) {
	blindSigs := []cashu.BlindSignature{}

	if sigs == nil {
		return blindSigs, nil
	}

	blindSigs = []cashu.BlindSignature{}

	for _, val := range sigs.GetSigs().BlindSignatures {
		dleq := cashu.BlindSignatureDLEQ{
			E: secp256k1.PrivKeyFromBytes(val.Dleq.E),
			S: secp256k1.PrivKeyFromBytes(val.Dleq.S),
		}

		C_, err := secp256k1.ParsePubKey(val.BlindedSecret)
		if err != nil {
			return blindSigs, fmt.Errorf("secp.secp256k1(ParsePubKey(val.BlindedSecret) %w", err)
		}
		blindSigs = append(blindSigs, cashu.BlindSignature{Amount: val.Amount, C_: cashu.WrappedPublicKey{PublicKey: C_}, Id: hex.EncodeToString(val.KeysetId), Dleq: &dleq})
	}

	return blindSigs, nil
}

func ConvertBlindedMessagedToGRPC(messages []cashu.BlindedMessage) (*sig.BlindedMessages, error) {
	messagesGrpc := sig.BlindedMessages{
		BlindedMessages: make([]*sig.BlindedMessage, len(messages)),
	}

	for i, val := range messages {
		B_ := val.B_.SerializeCompressed()

		idBytes, err := hex.DecodeString(val.Id)
		if err != nil {
			return &messagesGrpc, fmt.Errorf("hex.DecodeString(val.Id). %w", err)
		}

		messagesGrpc.BlindedMessages[i] = &sig.BlindedMessage{
			Amount:        val.Amount,
			KeysetId:      idBytes,
			BlindedSecret: B_,
			// Witness: &sig.Witness{} val.Witness,
		}
	}

	return &messagesGrpc, nil
}

func ConvertCashuUnitToSignature(unit cashu.Unit) (*sig.CurrencyUnit, error) {
	switch unit {
	case cashu.Sat:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_SAT}}, nil
	case cashu.Msat:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_MSAT}}, nil
	case cashu.EUR:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_EUR}}, nil
	case cashu.AUTH:
		return &sig.CurrencyUnit{CurrencyUnit: &sig.CurrencyUnit_Unit{Unit: sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_AUTH}}, nil

	default:
		return nil, fmt.Errorf("no available unit")
	}
}

func ConvertSigUnitToCashuUnit(sigUnit *sig.CurrencyUnit) (cashu.Unit, error) {
	switch sigUnit.GetUnit().Number() {
	case sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_SAT.Enum().Number():
		return cashu.Sat, nil
	case sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_MSAT.Enum().Number():
		return cashu.Msat, nil
	case sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_EUR.Enum().Number():
		return cashu.EUR, nil
	case sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_USD.Enum().Number():
		return cashu.USD, nil
	case sig.CurrencyUnitType_CURRENCY_UNIT_TYPE_AUTH.Enum().Number():
		return cashu.AUTH, nil

	default:
		unit, err := cashu.UnitFromString(strings.ToLower(sigUnit.GetCustomUnit()))

		if err != nil {
			return cashu.Sat, fmt.Errorf("cashu.UnitFromString(strings.ToLower(req.Unit.String())). %w", err)
		}
		return unit, nil
	}
}

// CheckIfSignerErrorExists maps gRPC error codes to application-specific errors
func CheckIfSignerErrorExists(err *sig.Error) error {
	if err == nil {
		return nil
	}

	var errResult error

	switch err.Code {
	case sig.ErrorCode_ERROR_CODE_AMOUNT_OUTSIDE_LIMIT:
		errResult = fmt.Errorf("%w: %s", cashu.ErrMessageAmountToBig, err.Detail)
	case sig.ErrorCode_ERROR_CODE_DUPLICATE_INPUTS_PROVIDED:
		errResult = fmt.Errorf("%w: %s", cashu.ErrRepeatedInput, err.Detail)
	case sig.ErrorCode_ERROR_CODE_DUPLICATE_OUTPUTS_PROVIDED:
		errResult = fmt.Errorf("%w: %s", cashu.ErrRepeatedOutput, err.Detail)
	case sig.ErrorCode_ERROR_CODE_KEYSET_NOT_KNOWN:
		errResult = fmt.Errorf("%w: %s", cashu.ErrKeysetNotFound, err.Detail)
	case sig.ErrorCode_ERROR_CODE_KEYSET_INACTIVE:
		errResult = fmt.Errorf("%w: %s", cashu.ErrUsingInactiveKeyset, err.Detail)
	case sig.ErrorCode_ERROR_CODE_MINTING_DISABLED:
		detail := err.Detail
		if detail == "" {
			detail = "Minting is disabled for this keyset"
		}
		// Using a custom error since there's no established error for this in the cashu package
		mintingDisabledErr := errors.New("minting disabled")
		errResult = fmt.Errorf("%w: %s", mintingDisabledErr, detail)
	case sig.ErrorCode_ERROR_CODE_COULD_NOT_ROTATE_KEYSET:
		errResult = fmt.Errorf("could not rotate keyset: %s", err.Detail)
	case sig.ErrorCode_ERROR_CODE_INVALID_PROOF:
		errResult = fmt.Errorf("%w: %s", cashu.ErrInvalidProof, err.Detail)
	case sig.ErrorCode_ERROR_CODE_INVALID_BLIND_MESSAGE:
		errResult = fmt.Errorf("%w: %s", cashu.ErrInvalidBlindMessage, err.Detail)
	case sig.ErrorCode_ERROR_CODE_UNIT_NOT_SUPPORTED:
		errResult = cashu.ErrUnitNotSupported
	case sig.ErrorCode_ERROR_CODE_UNSPECIFIED:
		detail := err.Detail
		if detail == "" {
			detail = "Unknown error occurred with the signer"
		}
		errResult = fmt.Errorf("%w, %s", cashu.ErrUnknown, detail)
	default:
		detail := err.Detail
		if detail == "" {
			detail = "Unspecified error happened with the signer"
		}
		errResult = fmt.Errorf("%w %s", cashu.ErrUnknown, detail)
	}

	return errResult
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
