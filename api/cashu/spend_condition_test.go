package cashu

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

const singleProofWithP2PK string = `{"amount":2,"C":"03952d912e6e8ba9f60c26a6120af9b50276b11b507aa09c66c3a5651c8521e819","id":"009a1f293253e41e","secret":"[\"P2PK\",{\"nonce\":\"ed8e7194f78cf3634e2dcf39e3fb8a263789cf9df3d5563347b8ce07c4c1f457\",\"data\":\"0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae\",\"tags\": [[\"sigflag\",\"SIG_ALL\"],[\"n_sigs\",\"2\"],[\"locktime\",\"1689418329\"],[\"refund\",\"033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e\"],[\"pubkeys\",\"02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904\",\"023192200a0cfd3867e48eb63b03ff599c7e46c8f4e41146b2d281173ca6c50c54\"]]}]","witness":"{\"signatures\":[\"83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d\"]}"}`

// this is the private key for public key: 0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae
const receiverPrivateKey string = "1f369c114315e02945ad9858f1e0e826013d0bfd5d294b274b530613a8975e75"
const MintPrivateKey string = "0000000000000000000000000000000000000000000000000000000000000001"
const RegtestRequest string = "lnbcrt10u1pnxrpvhpp535rl7p9ze2dpgn9mm0tljyxsm980quy8kz2eydj7p4awra453u9qdqqcqzzsxqyz5vqsp55mdr2l90rhluaz9v3cmrt0qgjusy2dxsempmees6spapqjuj9m5q9qyyssq863hqzs6lcptdt7z5w82m4lg09l2d27al2wtlade6n4xu05u0gaxfjxspns84a73tl04u3t0pv4lveya8j0eaf9w7y5pstu70grpxtcqla7sxq"

func TestParseProofWithP2PK(t *testing.T) {

	var proof Proof
	err := json.Unmarshal([]byte(singleProofWithP2PK), &proof)

	if err != nil {
		t.Fatalf("json.Unmarshal([]byte(singleProofWithP2PK)): %+v ", []byte(singleProofWithP2PK))
	}

	if proof.Witness != `{"signatures":["83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413"]}` {
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

	if hex.EncodeToString(spendCondition.Data.Data.SerializeCompressed()) != "0275c5c0ddafea52d669f09de48da03896d09962d6d4e545e94f573d52840f04ae" {
		t.Errorf("Error in spend condition data %+v", hex.EncodeToString(spendCondition.Data.Data.SerializeUncompressed()))
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeCompressed()) != "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904" {
		t.Errorf("Error in spend condition pubkey %+v", hex.EncodeToString(spendCondition.Data.Tags.Pubkeys[0].SerializeUncompressed()))
	}

	if hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeCompressed()) != "033281c37677ea273eb7183b783067f5244933ef78d8c3f15b1a77cb246099c26e" {
		t.Errorf("Error in spend condition refund %+v", hex.EncodeToString(spendCondition.Data.Tags.Refund[0].SerializeUncompressed()))
	}

	var p2pkWitness P2PKWitness
	// parse witness
	err = json.Unmarshal([]byte(proof.Witness), &p2pkWitness)

	if err != nil {
		t.Errorf("could not pass P2PKWitness %+v \n\n", err)
	}

	if hex.EncodeToString(p2pkWitness.Signatures[0].Serialize()) != "83b585b5d719e95c1cef8514b14b3a027a2053fe174a1b693051c6e2dcbcf6478b4759e5a25a36a0fd67eae392b3a73afa6677b80d1edbbb6b0a9837ef8c413d" {

		t.Errorf("Error in p2pkWitness[0] %+v", p2pkWitness.Signatures[0])

	}
}
