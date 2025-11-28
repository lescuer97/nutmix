package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const singleProofWithP2PK string = `{"amount":2,"C":"03952d912e6e8ba9f60c26a6120af9b50276b11b507aa09c66c3a5651c8521e819","id":"009a1f293253e41e","secret":"[\"P2PK\",{\"nonce\":\"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457\",\"data\":\"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae\",\"tags\": [[\"sigflag\",\"SIG_ALL\"],[\"n_sigs\",\"2\"],[\"locktime\",\"1689418329\"],[\"refund\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"],[\"pubkeys\",\"02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904\",\"023192200a0cfd3867e48eb63b03ff599c7e46c8f4e41146b2d281173ca6c50c54\"]]}]","witness":"{\"signatures\":[\"83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d\"]}"}`

// this is the private key for public key: 0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae
const receiverPrivateKey string = "1f369c114315e02945ad9858f1e0e826013d0bfd5d294b274b530613a8975e75"
const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"
const RegtestRequest string = "lnbcrt10u1pnxrpvhpp535rl7p9ze2dpgn9mm0tljyxsm980quy8kz2eydj7p4awra453u9qdqqcqzzsxqyz5vqsp55mdr2l90rhluaz9v3cmrt0qgjusy2dxsempmees6spapqjuj9m5q9qyyssq863hqzs6lcptdt7z5w82m4lg09l2d27al2wtlade6n4xu05u0gaxfjxspns84a73tl04u3t0pv4lveya8j0eaf9w7y5pstu70grpxtcqla7sxq"

var correctPreimage string
var IncorrectPreimage string

// there is a conditional check that the preimage is 32 bytes long
func init() {
	sum := sha256.Sum256([]byte("12345"))
	correctPreimage = hex.EncodeToString(sum[:])

	sum2 := sha256.Sum256([]byte("54321"))
	IncorrectPreimage = hex.EncodeToString(sum2[:])
}

func TestParseProofWithP2PK(t *testing.T) {

	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	if proof.Witness != `{"signatures":["83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d"]}` {
		t.Errorf("incorrect Witness: %s", proof.Witness)
	}

	if proof.Secret != `["P2PK",{"nonce":"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457","data":"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae","tags": [["sigflag","SIG_ALL"],["n_sigs","2"],["locktime","1689418329"],["refund","033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e"],["pubkeys","02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904","023192200a0cfd3867e48eb63b03ff599c7e46c8f4e41146b2d281173ca6c50c54"]]}]` {
		t.Errorf("incorrect Secret %s", proof.Secret)
	}

	// parse proof secret to golang data struct
	var spendCondition SpendCondition

	err = json.Unmarshal([]byte(proof.Secret), &spendCondition)

	if err != nil {
		t.Errorf("could not parse spend condition %+v \n\n", err)
	}

	if spendCondition.Type != P2PK {
		t.Errorf("Error in spend condition type %+v", spendCondition.Type)
	}

	if spendCondition.Data.Data != "0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae" {
		t.Errorf("Error in spend condition data %+v", spendCondition.Data.Data)
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeCompressed()) != "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904" {
		t.Errorf("Error in spend condition pubkey %+v", hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeUncompressed()))
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeCompressed()) != "033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e" {
		t.Errorf("Error in spend condition refund %+v", hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeUncompressed()))
	}

	var p2pkWitness Witness
	// parse witness
	err = json.Unmarshal([]byte(proof.Witness), &p2pkWitness)

	if err != nil {
		t.Errorf("could not pass P2PKWitness %+v \n\n", err)
	}

	if hex.EncodeToString(p2pkWitness.Signatures[0].Serialize()) != "83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d" {

		t.Errorf("Error in p2pkWitness[0] %+v", p2pkWitness.Signatures[0])

	}
}

const singleProofWithHTLC string = `{"amount":2,"C":"03952d912e6e8ba9f60c26a6120af9b50276b11b507aa09c66c3a5651c8521e819","id":"009a1f293253e41e","secret":"[\"HTLC\",{\"nonce\":\"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457\",\"data\":\"5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5\",\"tags\": [[\"sigflag\",\"SIG_INPUTS\"],[\"n_sigs\",\"1\"],[\"locktime\",\"16894183290000\"],[\"refund\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"],[\"pubkeys\",\"0375c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae\",\"023192200a0cfd3867e48eb63b03ff599c7e46c8f4e41146b2d281173ca6c50c54\"]]}]"}`

func TestParseProofWithHTLC(t *testing.T) {
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithHTLC), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithHTLC))
	}

	// parse proof secret to golang data struct
	var spendCondition SpendCondition

	err = json.Unmarshal([]byte(proof.Secret), &spendCondition)

	if err != nil {
		t.Errorf("could not parse spend condition %+v \n\n", err)
	}

	if spendCondition.Type != HTLC {
		t.Errorf("Error in spend condition type %+v", spendCondition.Type)
	}

	if spendCondition.Data.Data != "5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5" {
		t.Errorf("Error in spend condition data %+v", spendCondition.Data.Data)
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeCompressed()) != "0375c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae" {
		t.Errorf("Error in spend condition pubkey %+v", hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeUncompressed()))
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeCompressed()) != "033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e" {
		t.Errorf("Error in spend condition refund %+v", hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeUncompressed()))
	}
}

func TestValidPreimageAndSignature(t *testing.T) {
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithHTLC), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithHTLC))
	}
	privKeyBytes, err := hex.DecodeString(receiverPrivateKey)
	if err != nil {
		t.Fatalf("could not decode private key %+v", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)

	err = proof.Sign(privKey)
	if err != nil {
		t.Errorf("could not sign proof %+v", err)
	}
	err = proof.AddPreimage(correctPreimage)
	if err != nil {
		t.Errorf("could not add preimage %+v", err)
	}

	// checkOutputs := false
	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, err := proof.IsProofSpendConditioned()

	if isProofLocked == false {
		t.Errorf("Error in isProofLocked %+v", isProofLocked)
	}

	ok, err := proof.VerifyP2PK(spendCondition)

	if !ok {
		t.Errorf("Error in ok %+v", ok)
	}
	if err != nil {
		t.Errorf("Error in err %+v", err)
	}
}

const WrongPrivkey string = "0000000000000000000000000000000000000000000000000000000000000002"

func TestInvalidSignatureAndValidPreimageHTLC(t *testing.T) {
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithHTLC), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithHTLC))
	}
	privKeyBytes, err := hex.DecodeString(receiverPrivateKey)
	if err != nil {
		t.Fatalf("could not decode private key %+v", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)

	err = proof.Sign(privKey)
	if err != nil {
		t.Errorf("could not sign proof %+v", err)
	}
	err = proof.AddPreimage(IncorrectPreimage)
	if err != nil {
		t.Errorf("could not add preimage %+v", err)
	}

	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, err := proof.IsProofSpendConditioned()

	if isProofLocked == false {
		t.Errorf("Error in isProofLocked %+v", isProofLocked)
	}

	ok, err := proof.VerifyHTLC(spendCondition)

	if ok {
		t.Errorf("Error in ok %+v", ok)
	}
	if !errors.Is(err, ErrInvalidPreimage) {
		t.Errorf("Error in err %+v", err)
	}
}
func TestValidSignatureAndInvalidPreimageHTLC(t *testing.T) {
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithHTLC), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithHTLC))
	}
	privKeyBytes, err := hex.DecodeString(WrongPrivkey)
	if err != nil {
		t.Fatalf("could not decode private key %+v", err)
	}
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)

	err = proof.Sign(privKey)
	if err != nil {
		t.Errorf("could not sign proof %+v", err)
	}
	err = proof.AddPreimage(correctPreimage)
	if err != nil {
		t.Errorf("could not add preimage %+v", err)
	}

	// check if a proof is locked to a spend condition and verifies it
	isProofLocked, spendCondition, err := proof.IsProofSpendConditioned()

	if isProofLocked == false {
		t.Errorf("Error in isProofLocked %+v", isProofLocked)
	}

	ok, err := proof.VerifyP2PK(spendCondition)

	if ok {
		t.Errorf("Error in ok %+v", ok)
	}
	if !errors.Is(err, ErrNoValidSignatures) {
		t.Errorf("Error in err %+v", err)
	}
}

// INFO: Testing test vectors for nut11

func TestVectorValidProof(t *testing.T) {

	var proof Proof
	proofString := `{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"859d4935c4907062a6297cf4e663e2835d90d97ecdd510745d32f6816323a41f\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"60f3c9b766770b46caac1d27e1ae6b77c8866ebaeba0b9489fe6a15a837eaa6fcd6eaa825499c72ac342983983fd3ba3a8a41f56677cc99ffd73da68b59e1383\"]}"
}`
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}
	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Fatalf("spendCondition.VerifySignatures(witness, proof.Secret): %+v ", []byte(proofString))
	}

	if valid != true {

		t.Error("proof should be valid")

	}

}

func TestVectorInvalidProofSignature(t *testing.T) {
	proofString := `{
    "amount": 1,
    "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
    "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
    "id": "009a1f293253e41e",
    "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\"]}"
}`
	var proof Proof
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v %+v ", []byte(singleProofWithP2PK), err)
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)

	if valid != false {
		t.Error("proof should be valid")
	}
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Error("Error should be ErrNotEnoughSignatures")
	}
}

func TestVectorValid2Signatures(t *testing.T) {
	var proof Proof
	proofString := `{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\",\"9a72ca2d4d5075be5b511ee48dbc5e45f259bcf4a4e8bf18587f433098a9cd61ff9737dc6e8022de57c76560214c4568377792d4c2c6432886cc7050487a1f22\"]}"
}`
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Fatalf("spendCondition.VerifySignatures(witness, proof.Secret): %+v ", []byte(proofString))
	}

	if valid != true {

		t.Error("proof should be valid")

	}
}

func TestVectorNotEnoughtSignatures(t *testing.T) {
	proofString := `{
  "amount": 1,
  "secret": "[\"P2PK\",{\"nonce\":\"0ed3fcb22c649dd7bbbdcca36e0c52d4f0187dd3b6a19efcc2bfbebb5f85b2a1\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"02142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "id": "009a1f293253e41e",
  "witness": "{\"signatures\":[\"83564aca48c668f50d022a426ce0ed19d3a9bdcffeeaee0dc1e7ea7e98e9eff1840fcc821724f623468c94f72a8b0a7280fa9ef5a54a1b130ef3055217f467b3\"]}"
}`
	var proof Proof
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v %+v ", []byte(singleProofWithP2PK), err)
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)

	if valid != false {
		t.Error("proof should be valid")
	}
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Error("Error should be ErrNotEnoughSignatures")
	}
}

func TestVectorRefundKeySpendable(t *testing.T) {
	var proof Proof
	proofString := `{
  "amount": 1,
  "id": "009a1f293253e41e",
  "secret": "[\"P2PK\",{\"nonce\":\"902685f492ef3bb2ca35a47ddbba484a3365d143b9776d453947dcbf1ddf9689\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"locktime\",\"21\"],[\"n_sigs\",\"2\"],[\"refund\",\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "witness": "{\"signatures\":[\"710507b4bc202355c91ea3c147c0d0189c75e179d995e566336afd759cb342bcad9a593345f559d9b9e108ac2c9b5bd9f0b4b6a295028a98606a0a2e95eb54f7\"]}"
}`
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if err != nil {
		t.Fatalf("spendCondition.VerifySignatures(witness, proof.Secret): %+v ", []byte(proofString))
	}

	if valid != true {

		t.Error("proof should be valid")

	}
}

func TestVectorRefundSigInvalidFromFuture(t *testing.T) {
	proofString := `{
  "amount": 1,
  "id": "009a1f293253e41e",
  "secret": "[\"P2PK\",{\"nonce\":\"64c46e5d30df27286166814b71b5d69801704f23a7ad626b05688fbdb48dcc98\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"locktime\",\"21\"],[\"n_sigs\",\"2\"],[\"refund\",\"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
  "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
  "witness": "{\"signatures\":[\"f661d3dc046d636d47cb3d06586da42c498f0300373d1c2a4f417a44252cdf3809bce207c8888f934dba0d2b1671f1b8622d526840f2d5883e571b462630c1ff\"]}"
}`
	var proof Proof
	err := json.Unmarshal([]byte(proofString), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v %+v ", []byte(singleProofWithP2PK), err)
	}

	spendCondition, err := proof.parseSpendCondition()
	if err != nil {
		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
	}

	valid, err := proof.VerifyP2PK(spendCondition)
	if valid != false {
		t.Error("proof should be valid")
	}
	if !errors.Is(err, ErrNoValidSignatures) {
		t.Errorf("Error should be ErrNotEnoughSignatures. %+v", err)
	}
}

// func TestVectorSamePubkeySignatureMultisig(t *testing.T) {
// 	proofString := `{"amount":1,"id":"009a1f293253e41e","secret":"[\"P2PK\",{\"nonce\":\"e434a9efbc5f65d144a620e368c9a6dc12c719d0ebc57e0c74f7341864dc449a\",\"data\":\"02a60c27104cf6023581e790970fc33994a320abe36e7ceed16771b0f8d76f0666\",\"tags\":[[\"pubkeys\",\"039c6a20a6ba354b7bb92eb9750716c1098063006362a1fa2afca7421f262d45c5\",\"0203eb2f7cd72a4f725d3327216365d2df18bb4bbc810522fd973c9af987e9b05b\"],[\"locktime\",\"1744876528\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]","C":"02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904","witness":"{\"signatures\":[\"3e9ff9e55c9eccb9e5aa0b6c62d54500b40d0eebadb06efcc8e76f3ce38e0923f956ec1bccb9080db96a17c1e98a1b857abfd1a56bb25670037cea3db1f73d81\",\"c5e29c38e60c4db720cf3f78e590358cf1291a06b9eadf77c1108ae84d533520c2707ffda224eb6a63fddaee9abd5ecf8f2cd263d2556950550e3061a5511f65\"]}"}`
// 	var proof Proof
// 	err := json.Unmarshal([]byte(proofString), &proof)
//
// 	if err != nil {
// 		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v %+v ", []byte(singleProofWithP2PK), err)
// 	}
//
// 	spendCondition, err := proof.parseSpendCondition()
// 	if err != nil {
// 		t.Fatalf("proof.parseSpendCondition(): %+v %+v", []byte(proofString), err)
// 	}
//
// 	if spendCondition.Data.Tags.NSigs != 2 {
// 		t.Error("nsig should be of value 2")
// 	}
// 	if len(spendCondition.Data.Tags.Pubkeys) != 2 {
// 		t.Error("pubkey tags should be of length 2")
// 	}
//
// 	valid, err := proof.VerifyP2PK(spendCondition)
// 	if valid != false {
// 		t.Error("proof should be valid")
// 	}
// 	if !errors.Is(err, ErrNotEnoughSignatures) {
// 		t.Errorf("Error should be ErrNotEnoughSignatures. %+v", err)
// 	}
// }

// =====================================================
// Marshalling and Constant Tests
// =====================================================

// TestSpendConditionTypeConstants verifies the correct iota values for SpendConditionType
func TestSpendConditionTypeConstants(t *testing.T) {
	if AnyOneCanSpend != 0 {
		t.Errorf("AnyOneCanSpend should be 0, got %d", AnyOneCanSpend)
	}
	if P2PK != 1 {
		t.Errorf("P2PK should be 1, got %d", P2PK)
	}
	if HTLC != 2 {
		t.Errorf("HTLC should be 2, got %d", HTLC)
	}
}

// TestTagsConstants verifies the correct iota values for Tags
func TestTagsConstants(t *testing.T) {
	if Sigflag != 1 {
		t.Errorf("Sigflag should be 1, got %d", Sigflag)
	}
	if Pubkeys != 2 {
		t.Errorf("Pubkeys should be 2, got %d", Pubkeys)
	}
	if NSigs != 3 {
		t.Errorf("NSigs should be 3, got %d", NSigs)
	}
	if Locktime != 4 {
		t.Errorf("Locktime should be 4, got %d", Locktime)
	}
	if Refund != 5 {
		t.Errorf("Refund should be 5, got %d", Refund)
	}
	if NSigRefund != 6 {
		t.Errorf("NSigRefund should be 6, got %d", NSigRefund)
	}
}

// TestSigFlagConstants verifies the correct iota values for SigFlag
func TestSigFlagConstants(t *testing.T) {
	if SigAll != 1 {
		t.Errorf("SigAll should be 1, got %d", SigAll)
	}
	if SigInputs != 2 {
		t.Errorf("SigInputs should be 2, got %d", SigInputs)
	}
}

// TestNSigRefundString verifies NSigRefund.String() returns "n_sigs_refund"
func TestNSigRefundString(t *testing.T) {
	if NSigRefund.String() != "n_sigs_refund" {
		t.Errorf("NSigRefund.String() should be 'n_sigs_refund', got '%s'", NSigRefund.String())
	}
}

// TestSpendConditionMarshalRoundTrip tests unmarshal -> marshal -> unmarshal produces consistent results
func TestSpendConditionMarshalRoundTrip(t *testing.T) {
	// Parse original P2PK secret
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proof)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	var original SpendCondition
	err = json.Unmarshal([]byte(proof.Secret), &original)
	if err != nil {
		t.Fatalf("failed to unmarshal spend condition: %v", err)
	}

	// Verify original parsed correctly
	if original.Type != P2PK {
		t.Errorf("expected P2PK type, got %d", original.Type)
	}
	if original.Data.Tags.Sigflag != SigAll {
		t.Errorf("expected SigAll flag, got %d", original.Data.Tags.Sigflag)
	}

	// Marshal back to JSON
	marshalled, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal spend condition: %v", err)
	}

	// Unmarshal the marshalled JSON
	var roundTrip SpendCondition
	err = json.Unmarshal(marshalled, &roundTrip)
	if err != nil {
		t.Fatalf("failed to unmarshal marshalled spend condition: %v", err)
	}

	// Verify round-trip consistency
	if roundTrip.Type != original.Type {
		t.Errorf("type mismatch: original=%d, roundTrip=%d", original.Type, roundTrip.Type)
	}
	if roundTrip.Data.Nonce != original.Data.Nonce {
		t.Errorf("nonce mismatch: original=%s, roundTrip=%s", original.Data.Nonce, roundTrip.Data.Nonce)
	}
	if roundTrip.Data.Data != original.Data.Data {
		t.Errorf("data mismatch: original=%s, roundTrip=%s", original.Data.Data, roundTrip.Data.Data)
	}
	if roundTrip.Data.Tags.Sigflag != original.Data.Tags.Sigflag {
		t.Errorf("sigflag mismatch: original=%d, roundTrip=%d", original.Data.Tags.Sigflag, roundTrip.Data.Tags.Sigflag)
	}
	if roundTrip.Data.Tags.NSigs != original.Data.Tags.NSigs {
		t.Errorf("n_sigs mismatch: original=%d, roundTrip=%d", original.Data.Tags.NSigs, roundTrip.Data.Tags.NSigs)
	}
	if roundTrip.Data.Tags.Locktime != original.Data.Tags.Locktime {
		t.Errorf("locktime mismatch: original=%d, roundTrip=%d", original.Data.Tags.Locktime, roundTrip.Data.Tags.Locktime)
	}
	if len(roundTrip.Data.Tags.Pubkeys) != len(original.Data.Tags.Pubkeys) {
		t.Errorf("pubkeys length mismatch: original=%d, roundTrip=%d", len(original.Data.Tags.Pubkeys), len(roundTrip.Data.Tags.Pubkeys))
	}
	if len(roundTrip.Data.Tags.Refund) != len(original.Data.Tags.Refund) {
		t.Errorf("refund length mismatch: original=%d, roundTrip=%d", len(original.Data.Tags.Refund), len(roundTrip.Data.Tags.Refund))
	}
}

// TestTagsInfoMarshalJSON tests TagsInfo marshalling produces correct format
func TestTagsInfoMarshalJSON(t *testing.T) {
	// Parse a proof with tags
	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proof)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	var sc SpendCondition
	err = json.Unmarshal([]byte(proof.Secret), &sc)
	if err != nil {
		t.Fatalf("failed to unmarshal spend condition: %v", err)
	}

	// Marshal tags
	tagsJSON, err := sc.Data.Tags.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal tags: %v", err)
	}

	// Verify it's valid JSON array
	var parsed [][]string
	err = json.Unmarshal(tagsJSON, &parsed)
	if err != nil {
		t.Fatalf("marshalled tags is not valid JSON array: %v", err)
	}

	// Verify sigflag tag exists and has correct format
	foundSigflag := false
	for _, tag := range parsed {
		if len(tag) >= 2 && tag[0] == "sigflag" {
			foundSigflag = true
			if tag[1] != "SIG_ALL" {
				t.Errorf("expected SIG_ALL, got %s", tag[1])
			}
		}
	}
	if !foundSigflag {
		t.Error("sigflag tag not found in marshalled output")
	}
}

// TestHasSigAllMethod tests the HasSigAll() method
func TestHasSigAllMethod(t *testing.T) {
	// Test with SIG_ALL
	var proofSigAll Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proofSigAll)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	var scSigAll SpendCondition
	err = json.Unmarshal([]byte(proofSigAll.Secret), &scSigAll)
	if err != nil {
		t.Fatalf("failed to unmarshal spend condition: %v", err)
	}

	if !scSigAll.HasSigAll() {
		t.Error("HasSigAll() should return true for SIG_ALL condition")
	}

	// Test with SIG_INPUTS
	var proofSigInputs Proof
	err = json.Unmarshal([]byte(singleProofWithHTLC), &proofSigInputs)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	var scSigInputs SpendCondition
	err = json.Unmarshal([]byte(proofSigInputs.Secret), &scSigInputs)
	if err != nil {
		t.Fatalf("failed to unmarshal spend condition: %v", err)
	}

	if scSigInputs.HasSigAll() {
		t.Error("HasSigAll() should return false for SIG_INPUTS condition")
	}
}

// TestAnyOneCanSpendDetection tests that non-JSON 64-byte secrets are identified correctly
func TestAnyOneCanSpendDetection(t *testing.T) {
	// Create a proof with a 64-byte hex secret (not JSON structured)
	plainSecret := "a3d98f6b2c1e4f5d8c7b6a9e0f1d2c3b4a5e6f7d8c9b0a1e2f3d4c5b6a7e8f9d"
	proofWithPlainSecret := Proof{
		Amount: 1,
		Id:     "009a1f293253e41e",
		Secret: plainSecret,
	}

	// IsProofSpendConditioned should return false for plain secrets
	isLocked, spendCondition, err := proofWithPlainSecret.IsProofSpendConditioned()
	if err != nil {
		// Plain secrets that aren't valid JSON should not return an error
		// They should simply return false, nil, nil
		t.Logf("Note: plain secret returned error (expected for non-JSON): %v", err)
	}

	if isLocked {
		t.Error("plain 64-byte secret should not be detected as spend conditioned")
	}
	if spendCondition != nil {
		t.Error("spendCondition should be nil for plain secret")
	}
}

// TestProofsHaveSigAll tests the ProofsHaveSigAll helper function
func TestProofsHaveSigAll(t *testing.T) {
	// Test with proof that has SIG_ALL
	var proofSigAll Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proofSigAll)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	proofs := Proofs{proofSigAll}
	hasSigAll, err := ProofsHaveSigAll(proofs)
	if err != nil {
		t.Fatalf("ProofsHaveSigAll returned error: %v", err)
	}
	if !hasSigAll {
		t.Error("ProofsHaveSigAll should return true for proofs with SIG_ALL")
	}

	// Test with proof that has SIG_INPUTS
	var proofSigInputs Proof
	err = json.Unmarshal([]byte(singleProofWithHTLC), &proofSigInputs)
	if err != nil {
		t.Fatalf("failed to unmarshal proof: %v", err)
	}

	proofsSigInputs := Proofs{proofSigInputs}
	hasSigAll, err = ProofsHaveSigAll(proofsSigInputs)
	if err != nil {
		t.Fatalf("ProofsHaveSigAll returned error: %v", err)
	}
	if hasSigAll {
		t.Error("ProofsHaveSigAll should return false for proofs with SIG_INPUTS")
	}

	// Test with mixed proofs - should return true if any has SIG_ALL
	mixedProofs := Proofs{proofSigInputs, proofSigAll}
	hasSigAll, err = ProofsHaveSigAll(mixedProofs)
	if err != nil {
		t.Fatalf("ProofsHaveSigAll returned error: %v", err)
	}
	if !hasSigAll {
		t.Error("ProofsHaveSigAll should return true if any proof has SIG_ALL")
	}
}

// TestSpendConditionTypeIsSpendConditioned tests the IsSpendConditioned helper method
func TestSpendConditionTypeIsSpendConditioned(t *testing.T) {
	if AnyOneCanSpend.IsSpendConditioned() {
		t.Error("AnyOneCanSpend should not be spend conditioned")
	}
	if !P2PK.IsSpendConditioned() {
		t.Error("P2PK should be spend conditioned")
	}
	if !HTLC.IsSpendConditioned() {
		t.Error("HTLC should be spend conditioned")
	}
}

// =====================================================
// Negative Tests - Invalid Inputs and Error Cases
// =====================================================

// TestInvalidSpendConditionTypeString tests unmarshalling with invalid type string
func TestInvalidSpendConditionTypeString(t *testing.T) {
	invalidJSON := `["INVALID_TYPE",{"nonce":"test","data":"test","tags":[]}]`
	var sc SpendCondition
	err := json.Unmarshal([]byte(invalidJSON), &sc)
	if err != nil {
		t.Fatalf("unmarshal should not fail for invalid type (should default to AnyOneCanSpend): %v", err)
	}
	// Should default to AnyOneCanSpend (0) for invalid types
	if sc.Type != AnyOneCanSpend {
		t.Errorf("expected AnyOneCanSpend for invalid type, got %d", sc.Type)
	}
}

// TestInvalidTagName tests unmarshalling with invalid tag name
func TestInvalidTagName(t *testing.T) {
	invalidTagsJSON := `[["invalid_tag","value"]]`
	var tags TagsInfo
	err := json.Unmarshal([]byte(invalidTagsJSON), &tags)
	if err == nil {
		t.Error("expected error for invalid tag name, got nil")
	}
	if !errors.Is(err, ErrInvalidTagName) {
		t.Errorf("expected ErrInvalidTagName, got %v", err)
	}
}

// TestInvalidSigFlagValue tests unmarshalling with invalid sigflag value
func TestInvalidSigFlagValue(t *testing.T) {
	invalidSigFlagJSON := `[["sigflag","INVALID_SIG_FLAG"]]`
	var tags TagsInfo
	err := json.Unmarshal([]byte(invalidSigFlagJSON), &tags)
	if err == nil {
		t.Error("expected error for invalid sigflag value, got nil")
	}
	if !errors.Is(err, ErrInvalidSigFlag) {
		t.Errorf("expected ErrInvalidSigFlag, got %v", err)
	}
}

// TestMalformedTagArray tests unmarshalling with malformed tag arrays
func TestMalformedTagArray(t *testing.T) {
	testCases := []struct {
		name      string
		json      string
		expectErr bool
	}{
		{
			name:      "empty tag array",
			json:      `[[]]`,
			expectErr: true,
		},
		{
			name:      "tag with only name, no value",
			json:      `[["sigflag"]]`,
			expectErr: true,
		},
		{
			name:      "sigflag with multiple values",
			json:      `[["sigflag","SIG_ALL","extra"]]`,
			expectErr: true,
		},
		{
			name:      "n_sigs with no value",
			json:      `[["n_sigs"]]`,
			expectErr: true,
		},
		{
			name:      "pubkeys with no values",
			json:      `[["pubkeys"]]`,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tags TagsInfo
			err := json.Unmarshal([]byte(tc.json), &tags)
			if tc.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestInvalidPublicKeyFormat tests unmarshalling with invalid public key hex strings
func TestInvalidPublicKeyFormat(t *testing.T) {
	testCases := []struct {
		name      string
		json      string
		expectErr bool
	}{
		{
			name:      "invalid hex in pubkeys",
			json:      `[["pubkeys","not_hex_string"]]`,
			expectErr: true,
		},
		{
			name:      "invalid hex in refund",
			json:      `[["refund","not_hex_string"]]`,
			expectErr: true,
		},
		{
			name:      "too short hex string",
			json:      `[["pubkeys","0123"]]`,
			expectErr: true,
		},
		{
			name:      "invalid public key bytes",
			json:      `[["pubkeys","0000000000000000000000000000000000000000000000000000000000000000"]]`,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tags TagsInfo
			err := json.Unmarshal([]byte(tc.json), &tags)
			if tc.expectErr && err == nil {
				t.Error("expected error for invalid public key, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestInvalidNumericValues tests unmarshalling with invalid numeric tag values
func TestInvalidNumericValues(t *testing.T) {
	testCases := []struct {
		name      string
		json      string
		expectErr bool
	}{
		{
			name:      "n_sigs with non-numeric value",
			json:      `[["n_sigs","not_a_number"]]`,
			expectErr: true,
		},
		{
			name:      "n_sigs_refund with non-numeric value",
			json:      `[["n_sigs_refund","not_a_number"]]`,
			expectErr: true,
		},
		{
			name:      "locktime with non-numeric value",
			json:      `[["locktime","not_a_number"]]`,
			expectErr: true,
		},
		{
			name:      "n_sigs with negative number string",
			json:      `[["n_sigs","-1"]]`,
			expectErr: true, // strconv.ParseUint fails on negative numbers
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tags TagsInfo
			err := json.Unmarshal([]byte(tc.json), &tags)
			if tc.expectErr && err == nil {
				t.Error("expected error for invalid numeric value, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestInvalidSpendConditionJSONStructure tests unmarshalling with invalid JSON structures
func TestInvalidSpendConditionJSONStructure(t *testing.T) {
	testCases := []struct {
		name      string
		json      string
		expectErr bool
		validate  func(*testing.T, SpendCondition) // Optional validation function
	}{
		{
			name:      "not an array",
			json:      `{"type":"P2PK"}`,
			expectErr: true,
		},
		{
			name:      "array with only one element",
			json:      `["P2PK"]`,
			expectErr: false, // JSON unmarshal succeeds but Data may be incomplete
			validate: func(t *testing.T, sc SpendCondition) {
				// Verify that unmarshalling with incomplete data doesn't crash
				if sc.Type != P2PK {
					t.Errorf("expected P2PK type, got %d", sc.Type)
				}
			},
		},
		{
			name:      "array with more than two elements",
			json:      `["P2PK",{"nonce":"test"},"extra"]`,
			expectErr: false, // JSON unmarshal may succeed but ignore extra elements
			validate: func(t *testing.T, sc SpendCondition) {
				// Verify that extra elements are ignored
				if sc.Type != P2PK {
					t.Errorf("expected P2PK type, got %d", sc.Type)
				}
			},
		},
		{
			name:      "second element not an object",
			json:      `["P2PK","not_an_object"]`,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var sc SpendCondition
			err := json.Unmarshal([]byte(tc.json), &sc)
			if tc.expectErr && err == nil {
				t.Error("expected error for invalid JSON structure, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectErr && err == nil && tc.validate != nil {
				tc.validate(t, sc)
			}
		})
	}
}

// TestMarshalAnyOneCanSpendError tests that marshalling AnyOneCanSpend returns an error
func TestMarshalAnyOneCanSpendError(t *testing.T) {
	sc := SpendCondition{
		Type: AnyOneCanSpend,
		Data: SpendConditionData{
			Nonce: "test",
			Data:  "test",
		},
	}

	_, err := sc.MarshalJSON()
	if err == nil {
		t.Error("expected error when marshalling AnyOneCanSpend, got nil")
	}
	if !errors.Is(err, ErrInvalidSpendCondition) {
		t.Errorf("expected ErrInvalidSpendCondition, got %v", err)
	}
}

// TestCheckValidWithTooManyPubkeys tests CheckValid with too many pubkeys
func TestCheckValidWithTooManyPubkeys(t *testing.T) {
	// Create a spend condition with more than 10 total pubkeys (pubkeys + refund)
	sc := SpendCondition{
		Type: P2PK,
		Data: SpendConditionData{
			Tags: TagsInfo{
				Pubkeys: make([]*btcec.PublicKey, 6),
				Refund:  make([]*btcec.PublicKey, 5), // Total = 11 > 10
			},
		},
	}

	// Create dummy pubkeys (we'll use nil for testing, but in real scenario they'd be valid)
	// For this test, we just check the count logic
	err := sc.CheckValid()
	if err == nil {
		t.Error("expected error for too many pubkeys, got nil")
	}
	if !errors.Is(err, ErrInvalidSpendCondition) {
		t.Errorf("expected ErrInvalidSpendCondition, got %v", err)
	}
}

// TestTagFromStringInvalid tests TagFromString with invalid tag names
func TestTagFromStringInvalid(t *testing.T) {
	invalidTags := []string{
		"",
		"invalid",
		"sig_flag",  // wrong format
		"pubkey",    // singular instead of plural
		"n_sig",     // singular instead of plural
		"lock_time", // wrong format
		"refunds",   // plural instead of singular
	}

	for _, tag := range invalidTags {
		t.Run(tag, func(t *testing.T) {
			result, err := TagFromString(tag)
			if err == nil {
				t.Errorf("expected error for tag '%s', got nil (result: %d)", tag, result)
			}
			if !errors.Is(err, ErrInvalidTagName) {
				t.Errorf("expected ErrInvalidTagName for tag '%s', got %v", tag, err)
			}
		})
	}
}

// TestSigFlagFromStringInvalid tests SigFlagFromString with invalid flag values
func TestSigFlagFromStringInvalid(t *testing.T) {
	invalidFlags := []string{
		"",
		"INVALID",
		"SIG_ALL_INPUTS",
		"SIG_NONE",
		"sig_all", // lowercase
		"Sig_All", // mixed case
	}

	for _, flag := range invalidFlags {
		t.Run(flag, func(t *testing.T) {
			result, err := SigFlagFromString(flag)
			if err == nil {
				t.Errorf("expected error for flag '%s', got nil (result: %d)", flag, result)
			}
		})
	}
}

// TestSpendConditionTypeStringInvalid tests String() with invalid type
func TestSpendConditionTypeStringInvalid(t *testing.T) {
	invalidType := SpendConditionType(999)
	str, err := invalidType.String()
	if err == nil {
		t.Errorf("expected error for invalid type, got nil (string: %s)", str)
	}
	if !errors.Is(err, ErrConvertSpendConditionToString) {
		t.Errorf("expected ErrConvertSpendConditionToString, got %v", err)
	}
}

// TestProofsHaveSigAllWithInvalidProof tests ProofsHaveSigAll with proof that fails to parse
func TestProofsHaveSigAllWithInvalidProof(t *testing.T) {
	// Create a proof with invalid secret that will cause parse error
	invalidProof := Proof{
		Amount: 1,
		Id:     "test",
		Secret: `["INVALID",{"malformed":true}]`, // Invalid structure
	}

	proofs := Proofs{invalidProof}
	_, err := ProofsHaveSigAll(proofs)
	// Should handle gracefully - either return false or error
	if err == nil {
		// If no error, should return false for invalid proof
		hasSigAll, _ := ProofsHaveSigAll(proofs)
		if hasSigAll {
			t.Error("ProofsHaveSigAll should return false for invalid proof")
		}
	}
}
