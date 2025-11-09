package cashu

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

// This are NUT-11 Test Vectors

func TestSwapRequestMsg(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"c7f280eb55c1e8564e03db06973e94bc9b666d9e1ca42ad278408fe625950303\",\"data\":\"030d8acedfe072c9fa449a1efe0817157403fbec460d8e79f957966056e5dd76c1\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c97ee3d1db41cf0a3ddb601724be8711a032950811bf326f8219c50c4808d3cd",
      "witness": "{\"signatures\":[\"ce017ca25b1b97df2f72e4b49f69ac26a240ce14b3690a8fe619d41ccc42d3c1282e073f85acd36dc50011638906f35b56615f24e4d03e8effe8257f6a808538\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v. %+v", swapRequest, err)
	}
	msg := swapRequest.makeSigAllMsg()
	if msg != `["P2PK",{"nonce":"c7f280eb55c1e8564e03db06973e94bc9b666d9e1ca42ad278408fe625950303","data":"030d8acedfe072c9fa449a1efe0817157403fbec460d8e79f957966056e5dd76c1","tags":[["sigflag","SIG_ALL"]]}]02c97ee3d1db41cf0a3ddb601724be8711a032950811bf326f8219c50c4808d3cd2038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39` {
		t.Errorf("Message is not correct %v", msg)
	}

	hashMessage := sha256.Sum256([]byte(msg))

	if hex.EncodeToString(hashMessage[:]) != "de7f9e3ca0fcc5ed3258fcf83dbf1be7fa78a5ed6da7bf2aa60d61e9dc6eb09a" {
		t.Errorf("hash message is wrong %v", msg)
	}

}

func TestSwapRequestValidSignature(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"c7f280eb55c1e8564e03db06973e94bc9b666d9e1ca42ad278408fe625950303\",\"data\":\"030d8acedfe072c9fa449a1efe0817157403fbec460d8e79f957966056e5dd76c1\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c97ee3d1db41cf0a3ddb601724be8711a032950811bf326f8219c50c4808d3cd",
      "witness": "{\"signatures\":[\"ce017ca25b1b97df2f72e4b49f69ac26a240ce14b3690a8fe619d41ccc42d3c1282e073f85acd36dc50011638906f35b56615f24e4d03e8effe8257f6a808538\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
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
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"fa6dd3fac9086c153878dec90b9e37163d38ff2ecf8b37db6470e9d185abbbae\",\"data\":\"033b42b04e659fed13b669f8b16cdaffc3ee5738608810cf97a7631d09bd01399d\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "024d232312bab25af2e73f41d56864d378edca9109ae8f76e1030e02e585847786",
      "witness": "{\"signatures\":[\"27b4d260a1186e3b62a26c0d14ffeab3b9f7c3889e78707b8fd3836b473a00601afbd53a2288ad20a624a8bbe3344453215ea075fc0ce479dd8666fd3d9162cc\"]}"
    },
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"4007b21fc5f5b1d4920bc0a08b158d98fd0fb2b0b0262b57ff53c6c5d6c2ae8c\",\"data\":\"033b42b04e659fed13b669f8b16cdaffc3ee5738608810cf97a7631d09bd01399d\",\"tags\":[[\"locktime\",\"122222222222222\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02417400f2af09772219c831501afcbab4efb3b2e75175635d5474069608deb641"
    }
  ],
  "outputs": [
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    },
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "03afe7c87e32d436f0957f1d70a2bca025822a84a8623e3a33aed0a167016e0ca5"
    },
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "02c0d4fce02a7a0f09e3f1bca952db910b17e81a7ebcbce62cd8dcfb127d21e37b"
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
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"04bfd885fc982d553711092d037fdceb7320fd8f96b0d4fd6d31a65b83b94272\",\"data\":\"0275e78025b558dbe6cb8fdd032a2e7613ca14fda5c1f4c4e3427f5077a7bd90e4\",\"tags\":[[\"pubkeys\",\"035163650bbd5ed4be7693f40f340346ba548b941074e9138b67ef6c42755f3449\",\"02817d22a8edc44c4141e192995a7976647c335092199f9e076a170c7336e2f5cc\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03866a09946562482c576ca989d06371e412b221890804c7da8887d321380755be",
      "witness": "{\"signatures\":[\"be1d72c5ca16a93c5a34f25ec63ce632ddc3176787dac363321af3fd0f55d1927e07451bc451ffe5c682d76688ea9925d7977dffbb15bd79763b527f474734b0\",\"669d6d10d7ed35395009f222f6c7bdc28a378a1ebb72ee43117be5754648501da3bedf2fd6ff0c7849ac92683538c60af0af504102e40f2d8daca8e08b1ca16b\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
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

func TestSwapRequestInvalidPubkeysAndRefundMixed(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"3e9253419a11f0a541dd6baeddecf8356fc864b5d061f12f05632bc3aee6b5c4\",\"data\":\"0343cca0e48ce9e3fdcddba4637ff8cdbf6f5ed9cfdf1873e63827e760f0ed4db5\",\"tags\":[[\"pubkeys\",\"0235e0a719f8b046cee90f55a59b1cdd6ca75ce23e49cbcd82c9e5b7310e21ebcd\",\"020443f98b356e021bae82bdfc05ff433cab21e27fca9ab7b0995aedb2e7aabc43\"],[\"locktime\",\"100\"],[\"n_sigs\",\"2\"],[\"refund\",\"026b432e62b041bf9cdae534203739c73fa506c9a2d6aa58a52bc601a1dec421e1\",\"02e3494a2e07e7f6e7d4567e0da7a563592bff1e121df2383667f15b83e9168a9e\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026c12ee3bffa5c617debcf823bf1af6a9b47145b699f2737bba3394f0893eb869",
      "witness": "{\"signatures\":[\"bfe884145ce6512331324321c3946dfd812428a53656b108b59d26559a186ba2ab45e5be9ce94e2dff0d09078e25ccb82d06a8b3a63cd3dc67065b8f77292776\",\"236e5cc9c30f85a893a29a4302e41e6f2015caef4229f28fa65e2f5c9d55515cc9a1852093a81a5095055d85fd55bf4da124e55354b56e0a39e83b58b0afc197\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    },
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "03afe7c87e32d436f0957f1d70a2bca025822a84a8623e3a33aed0a167016e0ca5"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}

	err = swapRequest.ValidateSigflag()
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Errorf("Error should be for invalid spend conditions %+v", err)
	}
}

func TestSwapRequestWithLockTimeAndEnoughtRefundKeys(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"9ea35553beb18d553d0a53120d0175a0991ca6109370338406eed007b26eacd1\",\"data\":\"02af21e09300af92e7b48c48afdb12e22933738cfb9bba67b27c00c679aae3ec25\",\"tags\":[[\"locktime\",\"1\"],[\"refund\",\"02637c19143c58b2c58bd378400a7b82bdc91d6dedaeb803b28640ef7d28a887ac\",\"0345c7fdf7ec7c8e746cca264bf27509eb4edb9ac421f8fbfab1dec64945a4d797\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03dd83536fbbcbb74ccb3c87147df26753fd499cc2c095f74367fff0fb459c312e",
      "witness": "{\"signatures\":[\"23b58ef28cd22f3dff421121240ddd621deee83a3bc229fd67019c2e338d91e2c61577e081e1375dbab369307bba265e887857110ca3b4bd949211a0a298805f\",\"7e75948ef1513564fdcecfcbd389deac67c730f7004f8631ba90c0844d3e8c0cf470b656306877df5141f65fd3b7e85445a8452c3323ab273e6d0d44843817ed\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
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

func TestSwapRequestHTLCLockedWithPublicKeyLocked(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"HTLC\",{\"nonce\":\"d730dd70cd7ec6e687829857de8e70aab2b970712f4dbe288343eca20e63c28c\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"0350cda8a1d5257dbd6ba8401a9a27384b9ab699e636e986101172167799469b14\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03ff6567e2e6c31db5cb7189dab2b5121930086791c93899e4eff3dda61cb57273",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"a4c00a9ad07f9936e404494fda99a9b935c82d7c053173b304b8663124c81d4b00f64a225f5acf41043ca52b06382722bd04ded0fbeb0fcc404eed3b24778b88\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
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

// The following is an invalid `SwapRequest` with an HTLC also locked to a public key, locktime and refund key. locktime is
// not expired but proof is signed with refund key.
func TestSwapRequestHtlcLockedWithRefundButInvalid(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"HTLC\",{\"nonce\":\"512c4045f12fdfd6f55059669c189e040c37c1ce2f8be104ed6aec296acce4e9\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"03ba83defd31c63f8841d188f0d41b5bb3af1bb3c08d0ba46f8f1d26a4d45e8cad\"],[\"locktime\",\"4854185133\"],[\"refund\",\"032f1008a79c722e93a1b4b853f85f38283f9ef74ee4c5c91293eb1cc3c5e46e34\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02207abeff828146f1fc3909c74613d5605bd057f16791994b3c91f045b39a6939",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"7816d57871bde5be2e4281065dbe5b15f641d8f1ed9437a3ae556464d6f9b8a0a2e6660337a915f2c26dce1453a416daf682b8fb593b67a0750fce071e0759b9\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
    },
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "B_": "03afe7c87e32d436f0957f1d70a2bca025822a84a8623e3a33aed0a167016e0ca5"
    }
  ]
}`)

	var swapRequest PostSwapRequest
	err := json.Unmarshal(swapRequestJson, &swapRequest)
	if err != nil {
		t.Fatalf("could not marshall PostSwapRequest %+v", swapRequest)
	}

	err = swapRequest.ValidateSigflag()
	if !errors.Is(err, ErrNotEnoughSignatures) {
		t.Errorf("Error should be for invalid spend conditions %+v", err)
	}
}

func TestSwapRequestHTLCLockedWithPublicKeyWithTimelockPassed(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"HTLC\",{\"nonce\":\"c9b0fabb8007c0db4bef64d5d128cdcf3c79e8bb780c3294adf4c88e96c32647\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"039e6ec7e922abb4162235b3a42965eb11510b07b7461f6b1a17478b1c9c64d100\"],[\"locktime\",\"1\"],[\"refund\",\"02ce1bbd2c9a4be8029c9a6435ad601c45677f5cde81f8a7f0ed535e0039d0eb6c\",\"03c43c00ff57f63cfa9e732f0520c342123e21331d0121139f1b636921eeec095f\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0344b6f1471cf18a8cbae0e624018c816be5e3a9b04dcb7689f64173c1ae90a3a5",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"98e21672d409cc782c720f203d8284f0af0c8713f18167499f9f101b7050c3e657fb0e57478ebd8bd561c31aa6c30f4cd20ec38c73f5755b7b4ddee693bca5a5\",\"693f40129dbf905ed9c8008081c694f72a36de354f9f4fa7a61b389cf781f62a0ae0586612fb2eb504faaf897fefb6742309186117f4743bcebcb8e350e975e2\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "B_": "038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39"
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
