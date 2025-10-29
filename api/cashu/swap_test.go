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
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"741391687d73ee334e80b3978252d8b4d1b4c2877b03e9350a41d48f9fa32215\",\"data\":\"03d732118ebbb5594c3d2c4ec216fc4ed95ecef96203a27bf8797e0e1fc4dfb47f\",\"tags\":[[\"pubkeys\",\"036698d3c69f5eec5ac85a4b6a16445d7fa7356ef99b038f2f7ef2b0163e1a2028\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "021e7b4c29ff17f1f36c12bfa3b7bc76118fc79102c675012145511abfbb989bec",
      "witness": "{\"signatures\":[\"3834641aad79054b73c1384990486f2f2af9ef30288e0a13ee4e009ad781aad74eaa2bff0abc420c4e3bbd1f1484d3a28cb3380af7a0f84f1a6eab991ff47661\",\"fefd0725c508ed05c5f14ee8ef3cb859fe8b9c070c23c797d0b712dc3966063a1faa083a32eb8edc1a88a823fcc4784f64a32f604c0012833d25b630b7664b3a\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 1,
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
	if msg != `["P2PK",{"nonce":"741391687d73ee334e80b3978252d8b4d1b4c2877b03e9350a41d48f9fa32215","data":"03d732118ebbb5594c3d2c4ec216fc4ed95ecef96203a27bf8797e0e1fc4dfb47f","tags":[["pubkeys","036698d3c69f5eec5ac85a4b6a16445d7fa7356ef99b038f2f7ef2b0163e1a2028"],["n_sigs","2"],["sigflag","SIG_ALL"]]}]021e7b4c29ff17f1f36c12bfa3b7bc76118fc79102c675012145511abfbb989bec100bfa73302d12ffd038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39` {
		t.Errorf("Message is not correct %v", msg)
		// `[\"P2PK\",{\"nonce\":\"4375df06a8b865282598a3ecdb64c6bbf42f64275e8a2ac63b892e469ccc9cdf\",\"data\":\"026a51df04777126923dd27a67c19c7e33b808dbfaa6851aec9907424ac71e9eba\",\"tags\":[[\"pubkeys\",\"03d0d01a4116873f9c9d7bc294e2710889e92a5522028ba60dc68cc308a87b3d70\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]031933d1e24ea9020d29893594b7331b33c42068a8d8044ebbff5cf69201d6ba00100bfa73302d12ffd038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39`
		// ["P2PK",{"nonce":"4375df06a8b865282598a3ecdb64c6bbf42f64275e8a2ac63b892e469ccc9cdf","data":"026a51df04777126923dd27a67c19c7e33b808dbfaa6851aec9907424ac71e9eba","tags":[["pubkeys","03d0d01a4116873f9c9d7bc294e2710889e92a5522028ba60dc68cc308a87b3d70"],["n_sigs","2"],["sigflag","SIG_ALL"]]}]031933d1e24ea9020d29893594b7331b33c42068a8d8044ebbff5cf69201d6ba00100bfa73302d12ffd038ec853d65ae1b79b5cdbc2774150b2cb288d6d26e12958a16fb33c32d9a86c39
	}

}

func TestSwapRequestValidSignature(t *testing.T) {
	swapRequestJson := []byte(`{
  "inputs": [
    {
      "amount": 1,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"6507be98667717777e8a7b4f390f0ce3015ae55ab3d704515a58279dd29b0837\",\"data\":\"02340815f0b7e6aab8309359f2ebd23ecc3a77f391ad0f42429dea4a57726aabd5\",\"tags\":[[\"pubkeys\",\"02caa73a36330cd4dd1c35a601fccc5caf9ba0af9aaa32ff6fd997f8016958012e\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03800d22be5fc78ba23fb2c7a98c04ac4df18d5a347830492f8861123266128594",
      "witness": "{\"signatures\":[\"0517134f98154091ea9e9ff2b89124f7ea9f33808de6533ca4658f0cf71019d461305ee4029c7cd4f23eac8c6b8d19c0717a57250aa55c62a97cb5fecb62492e\",\"c129e6fdc3b90ad5de688551310aa8c8efc74d485ab699477e7dbb9e71d096b19535ae7ed8178e78016dad816fe83213693892e64e94b53caf63a6e1fb7fd90f\"]}"
    },
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"ec17595b7841d3f755a0511904d475406db0b55d87192f1249e8cba9c1af82d7\",\"data\":\"02340815f0b7e6aab8309359f2ebd23ecc3a77f391ad0f42429dea4a57726aabd5\",\"tags\":[[\"pubkeys\",\"02caa73a36330cd4dd1c35a601fccc5caf9ba0af9aaa32ff6fd997f8016958012e\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "025a0739fbff052ea7776ff84667d2f496073366b245bc1ed43ea51babba2ae83e"
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
      "secret": "[\"P2PK\",{\"nonce\":\"e2a221fe361f19d95c5c3312ccff3ffa075b4fe37beec99de85a6ee70568385b\",\"data\":\"03dad7f9c588f4cbb55c2e1b7b802fa2bbc63a614d9e9ecdf56a8e7ee8ca65be86\",\"tags\":[[\"pubkeys\",\"025f2af63fd65ca97c3bde4070549683e72769d28def2f1cd3d63576cd9c2ffa6c\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02a79c09b0605f4e7a21976b511cc7be01cdaeac54b29645258c84f2e74bff13f6",
      "witness": "{\"signatures\":[\"b42c7af7e98ca4e3bba8b73702120970286196340b340c21299676dbc7b10cafaa7baeb243affc01afce3218616cf8b3f6b4baaf4414fedb31b0c6653912f769\",\"17781910e2d806cae464f8a692929ee31124c0cd7eaf1e0d94292c6cbc122da09076b649080b8de9201f87d83b99fe04e33d701817eb287d1cdd9c4d0410e625\"]}"
    },
    {
      "amount": 2,
      "id": "00bfa73302d12ffd",
      "secret": "[\"P2PK\",{\"nonce\":\"973c78b5e84c0986209dc14ba57682baf38fa4c1ea60c4c5f6834779a1a13e6d\",\"data\":\"02685df03c777837bc7155bd2d0d8e98eede7e956a4cd8a9edac84532584e68e0f\",\"tags\":[[\"pubkeys\",\"025f2af63fd65ca97c3bde4070549683e72769d28def2f1cd3d63576cd9c2ffa6c\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02be48c564cf6a7b4d09fbaf3a78a153a79f687ac4623e48ce1788effc3fb1e024"
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
      "secret": "[\"P2PK\",{\"nonce\":\"9f130eaddeacec1e0cf67c4458b376316a566b797c9ab6ef2d90dc22da3244da\",\"data\":\"0258121ea454f256b310025855788a274eebd8e5f32c23db307388c7ac5f17669c\",\"tags\":[[\"pubkeys\",\"0273031aec7105bb1b1bed4320b22d2bfa26bf798a1f04103cc572b9c2ac31d629\",\"033980a7d123e67bd0cada4bb7463c3a1604d56da15f8bea00d93f2fa9fcb4ff03\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02f64662a77beef0df41d5ab3861c6816845b0ddbb535abfaa2d6cf1d67767db43",
      "witness": "{\"signatures\":[\"0a2e7934e3ee997553df0fad0d54c6c24dc398c0f1bd84f83dfafb55a57d60f82f426b4b5aadf12bbe3e729396bdac04260cb88ed720e05a483d7a3cfe5e060a\",\"5267f545d2da679d6e08ce453ab335f90c5cfd34b66f67cd078f6ad757a257e5471342701ec7b3f864c0326223ef4c92fa46efc5d4c94e7802844c51927265f7\"]}"
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
      "secret": "[\"P2PK\",{\"nonce\":\"cc93775c74df53d7c97eb37f72018d166a45ce4f4c65f11c4014b19acd02bd2f\",\"data\":\"02f515ab63e973e0dadfc284bf2ef330b01aa99c3ff775d88272f9c17afa25568c\",\"tags\":[[\"pubkeys\",\"026925e5bb547a3ec6b2d9b8934e23b882f54f89b2a9f45300bf81fd1b311d9c97\"],[\"n_sigs\",\"2\"],[\"refund\",\"03c8cd46b7e6592c41df38bc54dce2555586e7adbb15cc80a02d1a05829677286d\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03f6d40d0ab11f4082ee7e977534a6fcd151394d647cde4ab122157e6d755410fd",
      "witness": "{\"signatures\":[\"a9f61c2b7161a50839bf7f3e2e1cb9bd7bdacd2ce62c0d458a5969db44646dad409a282241b412e8b191cc7432bcfebf16ad72339a9fb966ca71c8bd971662cc\",\"aa778ec15fe9408e1989c712c823e833f33d45780b9a25555ea76004b05d495e99fd326914484f92e7e91f919ee575e79add26e9d4bbe4349d7333d7e0021af7\"]}"
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
      "secret": "[\"P2PK\",{\"nonce\":\"3ab4fe4969edd99ee9f3d40d2f382157ae5f382ba280ee5ff2d87360e315951b\",\"data\":\"032d3eecd23c9e50972d2964aaae2d302ffdb8717018469f05b051502191c398b1\",\"tags\":[[\"locktime\",\"1\"],[\"n_sigs\",\"1\"],[\"refund\",\"02d3edfb9e9ffdcd4845ba1d3f4cfc65503937c5c9d653ce49f315e76b608a8683\",\"03068b44ca2edca02b6e0832a9e014e409a5e44501e07d7227877efdf10aedf19d\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0244bb030c94f79092eb66bc84937ab920360fec3c333f424248592113fcc96cd6",
      "witness": "{\"signatures\":[\"83c45281c4a4dbbaab82c795ff435468f8c22506dc75debe34e5e07d1a889693e89ab1d621575039a1470bea1bf9a73dcf57f9902bff32afb52c4c403c852e46\",\"071570a852228cb16368807024fd6d7c53b1c3b1a574f206fd2cb6fd61235ad894be111a49a42133c786c366d0a96bfc108b45f6bcfa5496701e0d5cc2e4d86a\"]}"
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
      "secret": "[\"HTLC\",{\"nonce\":\"247864413ecc86739078f8ab56deb8006f9c304fc270f20eb30340beca173088\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"03f2a205a6468f29af3948f036e8e35e0010832d8d0b43b0903331263a45f93f31\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0394ffcb2ec2a96fd58c1b935784a7709c62954f7f11f1e684de471f808ccfb0bf",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"fa820534d75faac577eb5b42e9929a9f648baaaf28876cbcb7c10c6047cf97f6197d1cbf4907d94c1e77decf4b1acf0c85816a30524ee1b546181a19b838b535\"]}"
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
      "secret": "[\"HTLC\",{\"nonce\":\"b6f0c59ea4084369d4196e1318477121c2451d59ae767060e083cb6846e6bbe0\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"0329fdfde4becf9ff871129653ff6464bb2c922fbcba442e6166a8b5849599604f\"],[\"locktime\",\"4854185133\"],[\"refund\",\"035fcf4a5393e4bdef0567aa0b8a9555edba36e5fcb283f3bbce52d86a687817d3\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "024fbbee3f3cc306a48841ba327435b64de20b8b172b98296a3e573c673d52562b",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"7526819070a291f731e77acfbe9da71ddc0f748fd2a3e6c2510bc83c61daaa656df345afa3832fe7cb94352c8835a4794ad499760729c0be29417387d1fc3cd1\"]}"
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
      "secret": "[\"HTLC\",{\"nonce\":\"d4e089a466a5dd15031a406a3733adecf6f82aa76eee31d6bc8eaff3d78f6daa\",\"data\":\"ec4916dd28fc4c10d78e287ca5d9cc51ee1ae73cbfde08c6b37324cbfaac8bc5\",\"tags\":[[\"pubkeys\",\"0367ec6c26c688ddd6162907298726c6d5ad669f99cf27b3ac6240c64fa7c5036f\"],[\"locktime\",\"1\"],[\"refund\",\"0302208be01ac255b9845e88a571120d2ce2df3f877414a430e17b5c0d993b66de\",\"0275a814c7a891f3241aca84097253cd173b933d012009b1335a981599bec3cb3f\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0374419050d909ba80122ed5e1e8ae6cc569c269fdff257fc5eae32945ca6076fe",
      "witness": "{\"preimage\":\"0000000000000000000000000000000000000000000000000000000000000001\",\"signatures\":[\"4c7d55d6447c6d950fe2d2629441e8e69368be6e0f576bc4f343e830bcdc1e2296ddce74cb5a64245639464814ca98b129b06461b0897b0d1b94133050f233bd\",\"bb7fd77512ac69a47462e91c5e47e20b5ad1466d28ea71ffbdf5d0ae40d2865b90ffc34fc3202f3b775b9428667c9aa54d778af2055a530946db3a0311a28493\"]}"
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
