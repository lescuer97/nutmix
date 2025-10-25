package cashu

import (
	"encoding/json"
	"testing"
)

// This are NUT-11 Test Vectors
func TestMeltRequestMsg(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "uHwJ-f6HFAC-lU2dMw0KOu6gd5S571FXQQHioYMD",
  "inputs": [
    {
      "amount": 4,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"f5c26c928fb4433131780105eac330338bb9c0af2b2fd29fad9e4f18c4a96d84\",\"data\":\"03c4840e19277822bfeecf104dcd3f38d95b33249983ac6fed755869f23484fb2a\",\"tags\":[[\"pubkeys\",\"0256dcc53d9330e0bc6e9b3d47c26287695aba9fe55cafdde6f46ef56e09582bfb\"],[\"n_sigs\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02174667f98114abeb741f4964bdc88a3b86efde0afa38f791094c1e07e5df3beb",
      "witness": "{\"signatures\":[\"abeeceba92bc7d1c514844ddb354d1e88a9776dfb55d3cdc5c289240386e401c3d983b68371ce5530e86c8fc4ff90195982a262f83fa8a5335b43e75af5f5fc7\"]}"
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
	if msg != `["P2PK",{"nonce":"f5c26c928fb4433131780105eac330338bb9c0af2b2fd29fad9e4f18c4a96d84","data":"03c4840e19277822bfeecf104dcd3f38d95b33249983ac6fed755869f23484fb2a","tags":[["pubkeys","0256dcc53d9330e0bc6e9b3d47c26287695aba9fe55cafdde6f46ef56e09582bfb"],["n_sigs","1"],["sigflag","SIG_ALL"]]}]02174667f98114abeb741f4964bdc88a3b86efde0afa38f791094c1e07e5df3beb000bfa73302d12ffd038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39uHwJ-f6HFAC-lU2dMw0KOu6gd5S571FXQQHioYMD` {
		t.Errorf("Message is not correct %v", msg)

	}

}

func TestMeltRequestValidSignature(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "uHwJ-f6HFAC-lU2dMw0KOu6gd5S571FXQQHioYMD",
  "inputs": [
    {
      "amount": 4,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"f5c26c928fb4433131780105eac330338bb9c0af2b2fd29fad9e4f18c4a96d84\",\"data\":\"03c4840e19277822bfeecf104dcd3f38d95b33249983ac6fed755869f23484fb2a\",\"tags\":[[\"pubkeys\",\"0256dcc53d9330e0bc6e9b3d47c26287695aba9fe55cafdde6f46ef56e09582bfb\"],[\"n_sigs\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02174667f98114abeb741f4964bdc88a3b86efde0afa38f791094c1e07e5df3beb",
      "witness": "{\"signatures\":[\"abeeceba92bc7d1c514844ddb354d1e88a9776dfb55d3cdc5c289240386e401c3d983b68371ce5530e86c8fc4ff90195982a262f83fa8a5335b43e75af5f5fc7\"]}"
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
  "quote": "wYHbJm5S1GTL28tDHoUAwcvb-31vu5kfDhnLxV9D",
  "inputs": [
    {
      "amount": 4,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"1705e988054354b703bc9103472cc5646ec76ed557517410186fa827c19c444d\",\"data\":\"024c8b5ec0e560f1fc77d7872ab75dd10a00af73a8ba715b81093b800849cb21fb\",\"tags\":[[\"pubkeys\",\"028d32bc906b3724724244812c450f688c548020f5d5a8c1d6cd1075650933d1a3\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02f2a0ff12c4dd95f2476662f1df49e5126f09a5ea1f3ce13b985db57661953072",
      "witness": "{\"signatures\":[\"a98a2616716d7813394a54ddc82234e5c47f0ddbddb98ccd1cad25236758fa235c8ae64d9fccd15efbe0ad5eba52a3df8433e9f1c05bc50defcb9161a5bd4bc4\",\"dd418cbbb23276dab8d72632ee77de730b932a3c6e8e15bc8802cef13db0b346915fe6e04e7fae03c3b5af026e25f71a24dc05b28135f0a9b69bc6c7289b6b8d\"]}"
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
