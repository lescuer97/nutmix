package cashu

import (
	"encoding/json"
	"testing"
)

// This are NUT-11 Test Vectors
func TestMeltRequestMsg(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "2fc40ad3-2f6a-4a7e-91fb-b8c2b5dc2bf7",
  "inputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "secret": "[\"P2PK\",{\"nonce\":\"1d0db9cbd2aa7370a3d6e0e3ce5714758ed7a085e2f8da9814924100e1fc622e\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
      "witness": "{\"signatures\":[\"b2077717cfe43086582679ce3fbe1802f9b8652f93828c2e1a75b9e553c0ab66cd14b9c5f6c45a098375fe6583e106c7ccdb1421636daf893576e15815f3483f\",\"179f687c2236c3d0767f3b2af88478cad312e7f76183fb5781754494709334c578c7232dc57017d06b9130a406f8e3ece18245064cda4ef66808ed3ff68c933e\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "B_": "028b708cfd03b38bdc0a561008119594106f0c563061ae3fbfc8981b5595fd4e2b"
    }
  ]
}`)

	var meltRequest PostMeltBolt11Request
	err := json.Unmarshal(meltRequestJson, &meltRequest)
	if err != nil {
		t.Fatalf("could not marshall  %+v", meltRequestJson)
	}
	msg := meltRequest.makeSigAllMsg()
	if msg != `["P2PK",{"nonce":"1d0db9cbd2aa7370a3d6e0e3ce5714758ed7a085e2f8da9814924100e1fc622e","data":"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a","tags":[["pubkeys","026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a","03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9"],["n_sigs","2"],["sigflag","SIG_ALL"]]}]028b708cfd03b38bdc0a561008119594106f0c563061ae3fbfc8981b5595fd4e2b2fc40ad3-2f6a-4a7e-91fb-b8c2b5dc2bf7` {
		t.Errorf("Message is not correct %v", msg)

	}

}

func TestMeltRequestValidSignature(t *testing.T) {
	meltRequestJson := []byte(`{
  "quote": "0f983814-de91-46b8-8875-1b358a35298a",
  "inputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "secret": "[\"P2PK\",{\"nonce\":\"600050bd36cccdc71dec82e97679fa3e7712c22ea33cf4fe69d4d78223757e57\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
      "witness": "{\"signatures\":[\"b66c342654ccc95a62100f8f4a76afe1aea612c9c63383be3c7feb5110bb8eabe7ccaa9f117abd524be8c9a2e331e7d70248aeae337b9ce405625b3c49fc627d\"]}"
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
  "quote": "2fc40ad3-2f6a-4a7e-91fb-b8c2b5dc2bf7",
  "inputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "secret": "[\"P2PK\",{\"nonce\":\"1d0db9cbd2aa7370a3d6e0e3ce5714758ed7a085e2f8da9814924100e1fc622e\",\"data\":\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"tags\":[[\"pubkeys\",\"026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a\",\"03142715675faf8da1ecc4d51e0b9e539fa0d52fdd96ed60dbe99adb15d6b05ad9\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026f6a2b1d709dbca78124a9f30a742985f7eddd894e72f637f7085bf69b997b9a",
      "witness": "{\"signatures\":[\"b2077717cfe43086582679ce3fbe1802f9b8652f93828c2e1a75b9e553c0ab66cd14b9c5f6c45a098375fe6583e106c7ccdb1421636daf893576e15815f3483f\",\"179f687c2236c3d0767f3b2af88478cad312e7f76183fb5781754494709334c578c7232dc57017d06b9130a406f8e3ece18245064cda4ef66808ed3ff68c933e\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 0,
      "id": "009a1f293253e41e",
      "B_": "028b708cfd03b38bdc0a561008119594106f0c563061ae3fbfc8981b5595fd4e2b"
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
