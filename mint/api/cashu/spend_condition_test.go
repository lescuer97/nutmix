package cashu

import (
	"encoding/json"
	"fmt"
	"testing"
)

const singleProofWithP2PK string = `{"amount":2,"C":"03952d912e6e8ba9f60c26a6120af9b50276b11b507aa09c66c3a5651c8521e819","id":"009a1f293253e41e","secret":"[\"P2PK\",{\"nonce\":\"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457\",\"data\":\"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae\"}]","witness":"{\"signatures\":[\"83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d\"]}"}`

// this is the private key for public key: 0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae
const receiverPrivateKey string = "1f369c114315e02945ad9858f1e0e826013d0bfd5d294b274b530613a8975e75"
const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestParseProofWithP2PK(t *testing.T) {

	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	if proof.Witness != `{"signatures":["83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d"]}` {

		t.Errorf("incorrect Witness: %s", proof.Witness)

	}

	if proof.Secret != `["P2PK",{"nonce":"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457","data":"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae"}]` {
		t.Errorf("incorrect Secret %s", proof.Secret)
	}

	// parse proof secret to golang data struct
	var spendCondition SpendCondition

	err = json.Unmarshal([]byte(proof.Secret), &spendCondition)

	fmt.Printf("spendCondition %+v", spendCondition)

	if err != nil {
		t.Errorf("could not pass spend condition %+v \n\n", err)
	}

	if spendCondition.Type != P2PK {
		t.Errorf("Error in spend condition type %+v", spendCondition.Type)
	}

	// json.Unmarshal()

	// parse witness to golang data struct

}
