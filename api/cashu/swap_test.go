package cashu

import (
	"encoding/json"
	"errors"
	"testing"
)

// This are NUT-11 Test Vectors

func TestSwapRequestMsg(t *testing.T) {
	swapRequestJson := []byte(`{
						  "inputs": [
						    {
						      "amount": 0,
						      "id": "009a1f293253e41e",
						      "secret": "[\"P2PK\",{\"nonce\":\"c537ea76c1ac9cfa44d15dac91a63315903a3b4afa8e4e20f868f87f65ff2d16\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
						      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
						      "witness": "{\"signatures\":[\"c38cf7943f59206dc22734d39c17e342674a4025e6d3b424eb79d445a57257d57b45dd94fcd1b8dd8013e9240a4133bdef6523f64cd7288d890f3bbb8e3c6453\",\"f766dbb80e5c27de9a4770486e11e1bac0b1c4f782bf807a5189ea9c3e294559a3de4e217d3dfceafd4d9e8dcbfe4e9a188052d6dab9df07df7844224292de36\"]}"
						    }
						  ],
						  "outputs": [
						    {
						      "amount": 0,
						      "id": "009a1f293253e41e",
						      "B_": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a"
						    }
						  ]
						}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}
	msg := swapRequest.makeSigAllMsg()
	if msg != `["P2PK",{"nonce":"c537ea76c1ac9cfa44d15dac91a63315903a3b4afa8e4e20f868f87f65ff2d16","data":"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a","tags":[["pubkeys","03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9"],["n_sigs","2"],["sigflag","SIG_ALL"]]}]026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a` {
		t.Errorf("Message is not correct %v", msg)

	}

}

func TestSwapRequestValidSignature(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "secret": "[\"P2PK\",{\"nonce\":\"fc14ca312b7442d05231239d0e3cdcb6b2335250defcb8bec7d2efe9e26c90a6\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
      "witness": "{\"signatures\":[\"aa6f3b3f112ec3e834aded446ea67a90cdb26b43e08cfed259e0bbd953395c4af11117c58ec0ec3de404f31076692426cde40d2c1602d9dd067a872cb11ac3c0\"]}"
    },
    {
      "amount": 0,
      "id": "009a1f293253e41f",
      "secret": "[\"P2PK\",{\"nonce\":\"fc14ca312b7442d05231239d0e3cdcb6b2335250defcb8bec7d2efe9e26c90a6\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "B_": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}

	err = swapRequest.ValidateSigflag()
	if err != nil {
		t.Errorf("the should not have been an error while validating %+v", err)
	}
}

func TestSwapRequestInvalidMultipleSecrets(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 1,
      "secret": "[\"P2PK\",{\"nonce\":\"859d4935c4907062a6297cf4e663e2835d90d97ecdd510745d32f6816323a41f\",\"data\":\"0249098aa8b9d2fbec49ff8598feb17b592b986e62319a4fa488a3dc36387157a7\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
      "id": "009a1f293253e41e",
      "witness": "{\"signatures\":[\"60f3c9b766770b46caac1d27e1ae6b77c8866ebaeba0b9489fe6a15a837eaa6fcd6eaa825499c72ac342983983fd3ba3a8a41f56677cc99ffd73da68b59e1383\"]}"
    },
    {
      "amount": 1,
      "secret": "[\"P2PK\",{\"nonce\":\"859d4935c4907062a6297cf4e663e2835d90d97ecdd510745d32f6816323a41f\",\"data\":\"02a60c27104cf6023581e790970fc33994a320abe36e7ceed16771b0f8d76f0666\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
      "id": "009a1f293253e41f",
      "witness": "{\"signatures\":[\"60f3c9b766770b46caac1d27e1ae6b77c8866ebaeba0b9489fe6a15a837eaa6fcd6eaa825499c72ac342983983fd3ba3a8a41f56677cc99ffd73da68b59e1383\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "B_": "02698c4e2b5f9534cd0687d87513c759790cf829aa5739184a3e3735471fbda904",
      "id": "009a1f293253e41e"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}

	err = swapRequest.ValidateSigflag()
	if !errors.Is(err, ErrInvalidSpendCondition) {
		t.Errorf("Error should be for invalid spend conditions %+v", err)
	}
}

func TestSwapRequestValidMultiSig(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "secret": "[\"P2PK\",{\"nonce\":\"c537ea76c1ac9cfa44d15dac91a63315903a3b4afa8e4e20f868f87f65ff2d16\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
      "witness": "{\"signatures\":[\"c38cf7943f59206dc22734d39c17e342674a4025e6d3b424eb79d445a57257d57b45dd94fcd1b8dd8013e9240a4133bdef6523f64cd7288d890f3bbb8e3c6453\",\"f766dbb80e5c27de9a4770486e11e1bac0b1c4f782bf807a5189ea9c3e294559a3de4e217d3dfceafd4d9e8dcbfe4e9a188052d6dab9df07df7844224292de36\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "B_": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}

	err = swapRequest.ValidateSigflag()
	if err != nil {
		t.Errorf("there should not have been any error on multisig! %+v ", err)

	}
}
