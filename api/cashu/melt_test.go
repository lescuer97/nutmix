package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// This are NUT-11 Test Vectors
func TestMeltRequestMsg(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "cF8911fzT88aEi1d-6boZZkq5lYxbUSVs-HbJxK0",
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"bbf9edf441d17097e39f5095a3313ba24d3055ab8a32f758ff41c10d45c4f3de\",\"data\":\"029116d32e7da635c8feeb9f1f4559eb3d9b42d400f9d22a64834d89cde0eb6835\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02a9d461ff36448469dccf828fa143833ae71c689886ac51b62c8d61ddaa10028b",
      "witness": "{\"signatures\":[\"478224fbe715e34f78cb33451db6fcf8ab948afb8bd04ff1a952c92e562ac0f7c1cb5e61809410635be0aa94d0448f7f7959bd5762cc3802b0a00ff58b2da747\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    }
  ]
}`)

	var meltRequest PostMeltBolt11Request
	err := json.Unmarshal(meltRequestJson, &meltRequest)
	if err != nil {
		t.Fatalf("could not marshall  %+v", meltRequestJson)
	}
	msg := meltRequest.makeSigAllMsg()
	if msg != `["P2PK",{"nonce":"bbf9edf441d17097e39f5095a3313ba24d3055ab8a32f758ff41c10d45c4f3de","data":"029116d32e7da635c8feeb9f1f4559eb3d9b42d400f9d22a64834d89cde0eb6835","tags":[["sigflag","SIG_ALL"]]}]02a9d461ff36448469dccf828fa143833ae71c689886ac51b62c8d61ddaa10028b0038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39cF8911fzT88aEi1d-6boZZkq5lYxbUSVs-HbJxK0` {
		t.Errorf("Message is not correct %v", msg)
	}

	hashMessage := sha256.Sum256([]byte(msg))

	if hex.EncodeToString(hashMessage[:]) != "9efa1067cc7dc870f4074f695115829c3cd817a6866c3b84e9814adf3c3cf262" {
		t.Errorf("hash message is wrong %v", msg)
	}

}

func TestMeltRequestValidSignature(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "cF8911fzT88aEi1d-6boZZkq5lYxbUSVs-HbJxK0",
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"bbf9edf441d17097e39f5095a3313ba24d3055ab8a32f758ff41c10d45c4f3de\",\"data\":\"029116d32e7da635c8feeb9f1f4559eb3d9b42d400f9d22a64834d89cde0eb6835\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02a9d461ff36448469dccf828fa143833ae71c689886ac51b62c8d61ddaa10028b",
      "witness": "{\"signatures\":[\"478224fbe715e34f78cb33451db6fcf8ab948afb8bd04ff1a952c92e562ac0f7c1cb5e61809410635be0aa94d0448f7f7959bd5762cc3802b0a00ff58b2da747\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    }
  ]
}`)

	var meltRequest PostMeltBolt11Request
	err := json.Unmarshal(meltRequestJson, &meltRequest)
	if err != nil {
		t.Fatalf("could not marshall PostMeltRequest %+v", meltRequest)
	}

	err = meltRequest.ValidateSigflag()
	if err != nil {
		t.Errorf("the should not have been an error while validating %+v", err)
	}
}

func TestMeltRequestValidMultiSig(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "Db3qEMVwFN2tf_1JxbZp29aL5cVXpSMIwpYfyOVF",
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"68d7822538740e4f9c9ebf5183ef6c4501c7a9bca4e509ce2e41e1d62e7b8a99\",\"data\":\"0394e841bd59aeadce16380df6174cb29c9fea83b0b65b226575e6d73cc5a1bd59\",\"tags\":[[\"pubkeys\",\"033d892d7ad2a7d53708b7a5a2af101cbcef69522bd368eacf55fcb4f1b0494058\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03a70c42ec9d7192422c7f7a3ad017deda309fb4a2453fcf9357795ea706cc87a9",
      "witness": "{\"signatures\":[\"ed739970d003f703da2f101a51767b63858f4894468cc334be04aa3befab1617a81e3eef093441afb499974152d279e59d9582a31dc68adbc17ffc22a2516086\",\"f9efe1c70eb61e7ad8bd615c50ff850410a4135ea73ba5fd8e12a734743ad045e575e9e76ea5c52c8e7908d3ad5c0eaae93337e5c11109e52848dc328d6757a2\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    }
  ]
}`)

	var meltRequest PostMeltBolt11Request
	err := json.Unmarshal(meltRequestJson, &meltRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", meltRequest)
	}

	err = meltRequest.ValidateSigflag()
	if err != nil {
		t.Errorf("there should not have been any error on multisig! %+v ", err)

	}
}
