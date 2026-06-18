package cashu

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type compatProofEnvelope struct {
	Inputs Proofs `json:"inputs"`
}

var compatUnixTime *int64

func decodeProofsFromPayload(t *testing.T, payload []byte) Proofs {
	t.Helper()
	var envelope compatProofEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(proof payload): %v", err)
	}
	return envelope.Inputs
}

func decodeSwapRequest(t *testing.T, payload []byte) PostSwapRequest {
	t.Helper()
	var request PostSwapRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("json.Unmarshal(swap payload): %v", err)
	}
	return request
}

func decodeMeltRequest(t *testing.T, payload []byte) PostMeltBolt11Request {
	t.Helper()
	var request PostMeltBolt11Request
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("json.Unmarshal(melt payload): %v", err)
	}
	return request
}

func setCompatUnixTime(t *testing.T, unix int64) {
	t.Helper()
	compatUnixTime = &unix
	t.Cleanup(func() {
		compatUnixTime = nil
	})
}

func validateProofsWithLocalValidators(proofs Proofs) error {
	if compatUnixTime != nil {
		return validateProofsWithCompatTime(proofs, *compatUnixTime)
	}

	for _, proof := range proofs {
		isLocked, spendCondition, err := proof.IsProofSpendConditioned()
		if err != nil {
			return err
		}
		if !isLocked || spendCondition == nil {
			return ErrInvalidSpendCondition
		}

		switch spendCondition.Type {
		case P2PK:
			valid, err := proof.VerifyP2PK(spendCondition)
			if err != nil {
				return err
			}
			if !valid {
				return ErrInvalidSpendCondition
			}
		case HTLC:
			valid, err := proof.VerifyHTLC(spendCondition)
			if err != nil {
				return err
			}
			if !valid {
				return ErrInvalidSpendCondition
			}
		default:
			return ErrInvalidSpendCondition
		}

		if err := VerifyProofCondition(proof); err != nil {
			return err
		}
	}

	return VerifyProofsSpendConditions(proofs)
}

func validateProofsWithCompatTime(proofs Proofs, currentUnix int64) error {
	for _, proof := range proofs {
		if err := validateProofConditionWithCompatTime(proof, currentUnix); err != nil {
			return err
		}
	}
	return nil
}

func validateProofConditionWithCompatTime(proof Proof, currentUnix int64) error {
	isLocked, spendCondition, err := proof.IsProofSpendConditioned()
	if err != nil {
		return fmt.Errorf("proof.IsProofSpendConditioned(). %w", err)
	}
	if isLocked {
		if err := spendCondition.CheckValid(); err != nil {
			return fmt.Errorf("spendCondition.CheckValid(). %w", err)
		}
		switch spendCondition.Type {
		case P2PK:
			if err := validateP2PKProofWithCompatTime(proof, spendCondition, currentUnix); err != nil {
				return err
			}
		case HTLC:
			if err := validateHTLCProofWithCompatTime(proof, spendCondition, currentUnix); err != nil {
				return err
			}
		default:
			return ErrInvalidSpendCondition
		}
	}
	if !isLocked && len(proof.Secret) != 64 {
		return ErrCommonSecretNotCorrectSize
	}
	return nil
}

func validateP2PKProofWithCompatTime(proof Proof, spendCondition *SpendCondition, currentUnix int64) error {
	witness, err := proof.parseWitness()
	if err != nil {
		return fmt.Errorf("p.parseWitness(). %w", err)
	}
	if compatTimelockPassed(spendCondition, currentUnix) {
		valid, _ := proof.verifyTimelockPassedSpendCondition(spendCondition, witness)
		if valid {
			return nil
		}
	}
	valid, err := proof.verifyP2PKSpendCondition(spendCondition, witness)
	if err != nil {
		return fmt.Errorf("p.verifyP2PKSpendCondition(spendCondition, witness). %w", err)
	}
	if !valid {
		return ErrInvalidSpendCondition
	}
	return nil
}

func validateHTLCProofWithCompatTime(proof Proof, spendCondition *SpendCondition, currentUnix int64) error {
	witness, err := proof.parseWitness()
	if err != nil {
		return fmt.Errorf("p.parseWitness(). %w", err)
	}
	if compatTimelockPassed(spendCondition, currentUnix) {
		valid, _ := proof.verifyTimelockPassedSpendCondition(spendCondition, witness)
		if valid {
			return nil
		}
	}
	valid, err := proof.verifyHtlcSpendCondition(spendCondition, witness)
	if err != nil {
		return fmt.Errorf("p.verifyHtlcSpendCondition(spendCondition, witness). %w", err)
	}
	if !valid {
		return ErrInvalidSpendCondition
	}
	return nil
}

func compatTimelockPassed(spendCondition *SpendCondition, currentUnix int64) bool {
	return spendCondition.Data.Tags.Locktime != 0 && currentUnix > int64(spendCondition.Data.Tags.Locktime)
}

func validateSwapRequestForCompat(request PostSwapRequest) error {
	if compatUnixTime == nil {
		return request.ValidateSigflag()
	}
	return validateSwapRequestWithCompatTime(request, *compatUnixTime)
}

func validateSwapRequestWithCompatTime(request PostSwapRequest, currentUnix int64) error {
	sigAllCheck, err := checkForSigAll(request.Inputs)
	if err != nil {
		return fmt.Errorf("checkForSigAll(request.Inputs). %w", err)
	}
	if sigAllCheck.sigFlag == SigAll {
		firstProof := request.Inputs[0]
		firstSpendCondition, err := firstProof.parseSpendCondition()
		if err != nil {
			return fmt.Errorf("request.Inputs[0].parseSpendCondition(). %w", err)
		}
		firstWitness, err := firstProof.parseWitness()
		if err != nil {
			return fmt.Errorf("request.Inputs[0].parseWitness(). %w", err)
		}
		if firstSpendCondition == nil || firstWitness == nil {
			return ErrInvalidSpendCondition
		}
		if err := request.verifySigAllRepetition(); err != nil {
			return fmt.Errorf("request.verifySigAllRepetition(). %w", err)
		}
		msg := request.makeSigAllMsg()
		if err := checkSigAllProofValidWithCompatTime(msg, sigAllCheck, firstProof, currentUnix); err != nil {
			return fmt.Errorf("checkSigAllProofValidWithCompatTime(msg, sigAllCheck, firstProof, currentUnix). %w", err)
		}
	}
	return nil
}

func validateMeltRequestForCompat(request PostMeltBolt11Request) error {
	return request.ValidateSigflag()
}

func checkSigAllProofValidWithCompatTime(sigAllMsg string, sigAllValidation SigflagValidation, firstProof Proof, currentUnix int64) error {
	if sigAllValidation.sigFlag != SigAll {
		return fmt.Errorf("sigAllValidation has flag that is not SIG_ALL")
	}

	spendCondition, err := firstProof.parseSpendCondition()
	if err != nil {
		return fmt.Errorf("firstProof.parseSpendCondition(). %w", err)
	}
	witness, err := firstProof.parseWitness()
	if err != nil {
		return fmt.Errorf("firstProof.parseWitness(). %w", err)
	}
	if spendCondition == nil || witness == nil {
		return ErrInvalidSpendCondition
	}

	if compatTimelockPassed(spendCondition, currentUnix) {
		signatures, err := checkSigAllValidSignature(sigAllMsg, sigAllValidation.refundPubkeys, witness.Signatures)
		if err != nil {
			return fmt.Errorf("checkSigAllValidSignature(msg, refundPubkeys, witness.Signatures). %w", err)
		}
		if signatures >= sigAllValidation.signaturesRequiredRefund {
			return nil
		}
	}

	if sigAllValidation.proofType == HTLC {
		if err := spendCondition.VerifyPreimage(witness); err != nil {
			return fmt.Errorf("spendCondition.VerifyPreimage(witness). %w", err)
		}
	}

	signatures, err := checkSigAllValidSignature(sigAllMsg, sigAllValidation.pubkeys, witness.Signatures)
	if err != nil {
		return fmt.Errorf("checkSigAllValidSignature(msg, pubkeys, witness.Signatures). %w", err)
	}
	if signatures < sigAllValidation.signaturesRequired {
		return ErrNotEnoughSignatures
	}
	return nil
}

func assertCompatError(t *testing.T, err error, wantErr error) {
	t.Helper()
	if wantErr == nil {
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error %v, got success", wantErr)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
}
