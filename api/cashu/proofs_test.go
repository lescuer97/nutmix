package cashu

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestChangeProofsStateToPending(t *testing.T) {
	proofs := Proofs{
		Proof{
			Amount: 1,
			State:  PROOF_UNSPENT,
		},
		Proof{
			Amount: 2,
			State:  PROOF_UNSPENT,
		},
	}
	proofs.SetProofsState(PROOF_PENDING)

	if proofs[0].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
	if proofs[1].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
}

func TestChangeProofsStateToPendingAndQuoteSet(t *testing.T) {
	proofs := Proofs{
		Proof{
			Amount: 1,
			State:  PROOF_UNSPENT,
		},
		Proof{
			Amount: 2,
			State:  PROOF_UNSPENT,
		},
	}
	proofs.SetPendingAndQuoteRef("123")

	if proofs[0].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
	res := "123"
	if *proofs[0].Quote != res {
		t.Errorf("proof transformation not working, should be: %v. is:  ", "123")
	}
	if proofs[1].State != PROOF_PENDING {
		t.Errorf("proof transformation not working, should be: %v ", proofs[1].State)
	}
	if *proofs[1].Quote != res {
		t.Errorf("proof transformation not working, should be: %v ", "123")
	}
}

// NOTE: NUT-11 SIG_INPUTS Test Vectors

func TestCheckP2PKProof(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"859d4935c4907062a6297cf4e663e2835d90d97ecdd510745d32f6816323a41f\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"60f3c9b766770b46caac1d27e1ae6b77c8866ebaeba0b9489fe6a15a837eaa6fcd6eaa825499c72ac342983983fd3ba3a8a41f56677cc99ffd73da68b59e1383\"]}"
}`)
	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Errorf("should not have errored. %+v", err)

	}
	if !valid {

		t.Errorf("proof should have been valid")
	}

}

func TestCheckP2PKProofInvalidSignature(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\"]}"
}`)

	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Errorf("Error should be about no valid signatures. %+v", err)
	}
	if valid {
		t.Errorf("proof should have been valid")
	}
}

func TestCheckP2PKProofValidMultisig2of2(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\",\"9a72ca2d4d5075be5b511ee48dbc5e45f259bcf4a4e8bf18587f433098a9cd61ff9737dc6e8022de57c76560214c4568377792d4c2c6432886cc7050487a1f22\"]}"
}`)
	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Errorf("should not have errored. %+v", err)

	}
	if !valid {
		t.Errorf("proof should have been valid")
	}
}

func TestCheckP2PKProofInvalidMultisig(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\"]}"
}`)

	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Errorf("Error should be about no valid signatures. %+v", err)
	}
	if valid {
		t.Errorf("proof should have been valid")
	}
}

func TestCheckP2PKProofWithSpendableLocktime(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "id": "009a1f293253e41e",
  "secret": "[\"P2PK\",{\"nonce\":\"902685f492ef3bb2ca35a47ddbba484a3365d143b9776d453947dcbf1ddf9689\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"locktime\",\"21\"],[\"n_sigs\",\"2\"],[\"refund\",\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "witness": "{\"signatures\":[\"710507b4bc202355c91ea3c147c0d0189c75e179d995e566336afd759cb342bcad9a593345f559d9b9e108ac2c9b5bd9f0b4b6a295028a98606a0a2e95eb54f7\"]}"
}`)
	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}
	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Errorf("should not have errored. %+v", err)

	}
	if !valid {
		t.Errorf("proof should have been valid")
	}
}

func TestCheckP2PKProofInvalidLocktimeRefundKey(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 1,
  "id": "009a1f293253e41e",
  "secret": "[\"P2PK\",{\"nonce\":\"64c46e5d30df27286166814b71b5d69801704f23a7ad626b05688fbdb48dcc98\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"locktime\",\"21\"],[\"n_sigs\",\"2\"],[\"refund\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "witness": "{\"signatures\":[\"f661d3dc046d636d47cb3d06586da42c498f0300373d1c2a4f417a44252cdf3809bce207c8888f934dba0d2b1671f1b8622d526840f2d5883e571b462630c1ff\"]}"
}`)

	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Errorf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if !errors.Is(err, ErrNoValidSignatures) {
		t.Errorf("Error should be about no valid signatures. %+v", err)
	}
	if valid {
		t.Errorf("proof should have been valid")
	}
}

// NOTE: NUT-11 Multisig Test Vectors

// TestCheckP2PKProofValidLocktimeMultisig tests the multisig proof with locktime=21
// using 2 valid signatures from data + pubkeys set. This verifies that when locktime
// has passed, the proof can still be spent via the Locktime Multisig path (2-of-2 signatures).
func TestCheckP2PKProofValidLocktimeMultisig(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 64,
  "C": "02d7cd858d866fca404b5cb1ffd813946e6d19efa1af00d654080fd20266bdc0b1",
  "id": "001b6c716bf42c7e",
  "secret": "[\"P2PK\",{\"nonce\":\"395162bf2d0add3c66aea9f22c45251dbee6e04bd9282addbb366a94cd4fb482\",\"data\":\"03ab50a667926fac858bac540766254c14b2b0334d10e8ec766455310224bbecf4\",\"tags\":[[\"locktime\",\"21\"],[\"pubkeys\",\"0229a91adec8dd9badb228c628a07fc1bf707a9b7d95dd505c490b1766fa7dc541\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"],[\"n_sigs\",\"2\"],[\"refund\",\"03ab50a667926fac858bac540766254c14b2b0334d10e8ec766455310224bbecf4\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"]]}]",
  "witness": "{\"signatures\":[\"6a4dd46f929b4747efe7380d655be5cfc0ea943c679a409ea16d4e40968ce89de885d995937d5b85f24fa33a25df10990c5e11d5397199d779d5cf87d42f6627\",\"0c266fffe2ea2358fb93b5d30dfbcefe52a5bb53d6c85f37d54723613224a256165d20dd095768f168ab2e97bc5a879f7c2a84eee8963c9bcedcd39552dbe093\"]}"
}`)
	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Errorf("should not have errored. %+v", err)
	}
	if !valid {
		t.Errorf("proof should have been valid")
	}
}

// TestCheckP2PKProofValidRefundMultisig tests the same multisig proof but using
// 1 valid signature from the refund tag. This verifies the Refund Multisig path
// when locktime has passed.
func TestCheckP2PKProofValidRefundMultisig(t *testing.T) {
	proofJsonBytes := []byte(`{
  "amount": 64,
  "C": "02d7cd858d866fca404b5cb1ffd813946e6d19efa1af00d654080fd20266bdc0b1",
  "id": "001b6c716bf42c7e",
  "secret": "[\"P2PK\",{\"nonce\":\"395162bf2d0add3c66aea9f22c45251dbee6e04bd9282addbb366a94cd4fb482\",\"data\":\"03ab50a667926fac858bac540766254c14b2b0334d10e8ec766455310224bbecf4\",\"tags\":[[\"locktime\",\"21\"],[\"pubkeys\",\"0229a91adec8dd9badb228c628a07fc1bf707a9b7d95dd505c490b1766fa7dc541\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"],[\"n_sigs\",\"2\"],[\"refund\",\"03ab50a667926fac858bac540766254c14b2b0334d10e8ec766455310224bbecf4\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"]]}]",
  "witness": "{\"signatures\":[\"d39631363480adf30433ee25c7cec28237e02b4808d4143469d4f390d4eae6ec97d18ba3cc6494ab1d04372f0838426ea296f25cb4bd8bddb296adc292eeaa96\"]}"
}`)
	var proof Proof
	err := json.Unmarshal(proofJsonBytes, &proof)
	if err != nil {
		t.Fatalf("json.Unmarshal(proofJsonBytes, &proof) %+v", err)
	}
	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("could not parse spend condition. %+v", err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Errorf("should not have errored. %+v", err)
	}
	if !valid {
		t.Errorf("proof should have been valid")
	}
}
