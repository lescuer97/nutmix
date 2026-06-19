package cashu

import "testing"

func TestCompatibility_p2pk_swap_unsigned_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"072dd1bbb55aa17a9998fe89284c4555c523adb70239f00bdd91075f9c98055a\",\"data\":\"03648c241674db282ee7731b0f82b7d840972ce1e8236f610dfca0e67f007dddfe\"}]",
      "C": "03d865193f379a49e83fcc0226d2df30d4f49e84d9393f93074e7fee00d96d668f"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"dd94f0fbd27eb3353b60cc94e4043c2028b3e48e5c1c97e99a4d49691bdfcd62\",\"data\":\"03648c241674db282ee7731b0f82b7d840972ce1e8236f610dfca0e67f007dddfe\"}]",
      "C": "026cbeb8477ea448f88dfc3bf0e6463ebd1f2739ee31009d4f498c963bae49d8bd"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02ebb7a5486cf24920207d1fd0272ae35c589df20c1973c3070727e52a6a64a08b"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "026d65e40128568704d03510ca536ac29ccab98d41b619e992cc93705f1fc49236"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_swap_unsigned_fails label=unsigned_P2PK_spend\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_p2pk_partial_signatures_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"dc42f259986b9317d74011e02646cdb17b75c774c5060644acf38e400550291a\",\"data\":\"033b5930fc850660fdc78e83ab6c938ce341b232402261df02b258a511af1c5124\"}]",
      "C": "02a2abed79a6decbd61b9590a162b2e51b63c809af11530c20e1dd557b653a7765",
      "witness": "{\"signatures\":[\"afe259d70fbe0fbaa11249321cf689db27a43406395f954e5cb69456582051ab9a93badd6ff5761b029725d5ca13250b3b2836e9757aca166eb3a20726bb7188\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"d16098101afdc7ede91caea2535a39ee8ba098fb0e56769fa9de746fbebbc0d7\",\"data\":\"033b5930fc850660fdc78e83ab6c938ce341b232402261df02b258a511af1c5124\"}]",
      "C": "037b8829e23270f414aaa5e8079469671e00ba7d9d9e8b91dc564cb856d88c873c"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02ec1eab2ce74cb8114cfafeb45a3304cb2b4a5c1f7d8948b91fc4eea66bf47bae"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03b5c19534de02b7b0e10d88b9822ff3c81c1d23a622d0d8278d89e89aa2433d9e"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_partial_signatures_fail label=partial_signature_P2PK_spend\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_p2pk_swap_signed_succeeds(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"427160eeca48319454886202ee998efb8c9c65703823ad4418a09e51e05c3c0a\",\"data\":\"0353a091a76a5cdd23c87a1508141b14505c71d5d4bc57f59777127c89f82e845f\"}]",
      "C": "03ab9d11b29d0384ba645ac01f5846bba3748216344bdd11a525b4301f67bf9aa3",
      "witness": "{\"signatures\":[\"bfbe10277a8e932d186009d54e6d2817fb3a2dc174440a456fd8d04915632c9cff87d2ab98b738e0f2a82de750b11ac3c6b8d7fa4337fcf75bf428c579c33fed\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"4addc3664bc2a19a49a3340e07f244554b459dc446346a1878fe8bcd2f542dd2\",\"data\":\"0353a091a76a5cdd23c87a1508141b14505c71d5d4bc57f59777127c89f82e845f\"}]",
      "C": "02319644fcc2bc92a7c6ee21e337b746080a8250199a2f80fb400c340581f9b3e7",
      "witness": "{\"signatures\":[\"c94779eec6d1d42378afb6a775fa0454fb9bf060763b8aeab9f9347732a2f2716d067dca67b619226a2faf658191cc9e51a6c7a6baec8d63b53de27c6616b0b0\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03ee692551408bb2a7e419b40e5261a82032034e919acab462a880945fdfb187f7"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "022fce7da081365fc2c8c1b07c27af3a9f570c4daf309b9b30bb337c8f38916044"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_swap_signed_succeeds label=signed_P2PK_spend\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_multisig_2of3(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"179d809937cfde2900a8975acdef44bcf94939fcd4be81b0ab13da0db87bde63\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0306473db7ef6175c34af0726f802cf1f4ff083bb2fe563883a52c53387070e195",
      "witness": "{\"signatures\":[\"6f6cf63567eb74d230f05d3f32fd624160a61d9bc923975d1b98fc4536379c7ff38928294baba0d35616a254a7d2a67eeb6e1d99de43d9c66381c9997fbc4dcb\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0726e906644e9f1b586d052bd0911956a4cedbce0b6943fbf1b468b15636026f\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02b8b05f1775189302845f0dd1720a8f2964aee060c868dbf6bed237014e32083e",
      "witness": "{\"signatures\":[\"53c3aec2c5cfb633e3941b30458e8efdebce5527ba6d484ca2c5fbfbc4ec2790b14517dc60094ca8d7b9ed4e55d88288a31c054bb91ec17a27ab1293f41fd33a\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c3920124f64da2d698efdd0578fe20117310c74f53a847bd420858fc4de64877"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0231ef098089b6ec712698e1d0af9517e744f4dc79a4d91ae53bbeee74fbef2d06"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_multisig_2of3 label=1_of_2_multisig\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"179d809937cfde2900a8975acdef44bcf94939fcd4be81b0ab13da0db87bde63\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0306473db7ef6175c34af0726f802cf1f4ff083bb2fe563883a52c53387070e195",
      "witness": "{\"signatures\":[\"a96a4a91d04851a26548de82cef4ebe0dcb30b25973073aebaf4f473c89579d22cb7a7acf823b9a2f4b0108753795e932d36b65b289725da7211a3b509361f93\",\"3d6c37d2f33c7a55458baf945058fd69f531a3276d87bdbe8ca54af77125cc3fb2d3864c2e725c99045af4ec62629f4cee54ff2bbd8650f3391aa4b972dde6c2\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0726e906644e9f1b586d052bd0911956a4cedbce0b6943fbf1b468b15636026f\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02b8b05f1775189302845f0dd1720a8f2964aee060c868dbf6bed237014e32083e",
      "witness": "{\"signatures\":[\"1d3e6083099052aeb7f2aa4213be61ea7476085fd85881cc488e5186c253666e46330fa1bf3cb40fdc4a9bb766dab86e1840f5b0a1eee47c1f8dc6fe91f2583a\",\"ae33e04f204063f1b91ef24f12f7059addee0445cdc8b63b278115263c43f60526a40e75b9f5dab08b57070439cb412e6522516112b6b744b4d19264181f6ba7\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c3920124f64da2d698efdd0578fe20117310c74f53a847bd420858fc4de64877"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0231ef098089b6ec712698e1d0af9517e744f4dc79a4d91ae53bbeee74fbef2d06"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_multisig_2of3 label=invalid_multisig\n%s", payload2)
	proofs2 := decodeProofsFromPayload(t, payload2)
	err = validateProofsWithLocalValidators(proofs2)
	assertCompatError(t, err, ErrNoValidSignatures)

	payload3 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"179d809937cfde2900a8975acdef44bcf94939fcd4be81b0ab13da0db87bde63\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0306473db7ef6175c34af0726f802cf1f4ff083bb2fe563883a52c53387070e195",
      "witness": "{\"signatures\":[\"8f821eb524a41131cc488aa8eab821b0c157ce8cdd8d6a2dca6745e7d29e3aa5f31e5de05559da8cfad6d524d468389ed0ff31a2985e6953b21cfe4714f64f9d\",\"9e973ba2505c26137f79f11c9088d68bfdb0060c8ce07489d6b0a861eac84e7b7b199a9f96c58219b13410619628a443868d034b7c0e905b670a266ac4e78560\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0726e906644e9f1b586d052bd0911956a4cedbce0b6943fbf1b468b15636026f\",\"data\":\"034d254efea16189df43172be881000cfdac169e3fc333f6253eabe8f583d2610d\",\"tags\":[[\"pubkeys\",\"031042be3bce245f85e95e845f66ec212676d303b23a5a1328c53d1f8da6a13b07\",\"02d8c1562b1e56b9c313b420565b0d90fe5a54f1386bc96f8fe4e5d0878c3a7f81\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02b8b05f1775189302845f0dd1720a8f2964aee060c868dbf6bed237014e32083e",
      "witness": "{\"signatures\":[\"8fff33f982153d1ffb40ba4305512274083404390c53dd7d7342aea61056aa97dcbf9b353d136558751afa52bb812cd1c85218c63317e0d90799a0a9edb9c106\",\"dece91cf7b07b90734b987af3d573c831f8f061762863917f38b8099ce448619a13aa7b28dbb7560ddd85844d36ceef542fe09dbdfc8a9be7443f006538ddc2c\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c3920124f64da2d698efdd0578fe20117310c74f53a847bd420858fc4de64877"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0231ef098089b6ec712698e1d0af9517e744f4dc79a4d91ae53bbeee74fbef2d06"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_multisig_2of3 label=valid_multisig\n%s", payload3)
	proofs3 := decodeProofsFromPayload(t, payload3)
	err = validateProofsWithLocalValidators(proofs3)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_locktime_before_expiry_primary_only(t *testing.T) {
	setCompatUnixTime(t, 1781734543)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"055779983e8894fb8c05846d0385c09cb8af34c9aee0d4fc66eca330a210fb02\",\"data\":\"02ae6a02921408fac78719941f38a5c73e68771b2c19d361c4e82886f06e13f444\",\"tags\":[[\"locktime\",\"1781734544\"],[\"refund\",\"02afab98512efa342052a068ac3abd3e7b9281705dd0e4b322d1469817f063416d\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02bbe99f7e2b9178e8ee8b57ccaf1393ee0408f9c6317cd87399d96d1f75ed6058",
      "witness": "{\"signatures\":[\"69ab99768b53bb47ae9d46bf37aefdd214ade12871e882823ed6d93c3f6e4489862a5b10d2f593813a086b51935a03b60208b34a91100b77a98c42f2a1294327\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"68f3a6a8613b3509a9fde51322e2a360c4a67f6b778ebfa183b2170503b523c9\",\"data\":\"02ae6a02921408fac78719941f38a5c73e68771b2c19d361c4e82886f06e13f444\",\"tags\":[[\"locktime\",\"1781734544\"],[\"refund\",\"02afab98512efa342052a068ac3abd3e7b9281705dd0e4b322d1469817f063416d\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "028553d7190f007147658238bb89bda449d5aebadd9439f9a19addd6d625bc15e6",
      "witness": "{\"signatures\":[\"76fecf60ccb11b0d98aa38afb6657cdfb38e85fd59ec679993503c017beda846e631cb409ab2c504e3caac2837b69bd6d37d8550107d6b341cdffa219fffd714\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "032fcdd4b7626c0f0c3c6c914befc746ab989bbeb8a68bf80bc703edf8f4c817aa"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c5564ee85e29b392258c97c62068bb4ffcdcd8d527db366be5a62cf8445d5303"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_locktime_before_expiry_primary_only label=refund_before_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"055779983e8894fb8c05846d0385c09cb8af34c9aee0d4fc66eca330a210fb02\",\"data\":\"02ae6a02921408fac78719941f38a5c73e68771b2c19d361c4e82886f06e13f444\",\"tags\":[[\"locktime\",\"1781734544\"],[\"refund\",\"02afab98512efa342052a068ac3abd3e7b9281705dd0e4b322d1469817f063416d\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02bbe99f7e2b9178e8ee8b57ccaf1393ee0408f9c6317cd87399d96d1f75ed6058",
      "witness": "{\"signatures\":[\"f26d262a940792c72e04eee8e14d15713919ec2e3702e51dc30630e6b2874c1db042e43dd44adbf2925efc7767797aa42bfecad8df14017cc697079679326c53\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"68f3a6a8613b3509a9fde51322e2a360c4a67f6b778ebfa183b2170503b523c9\",\"data\":\"02ae6a02921408fac78719941f38a5c73e68771b2c19d361c4e82886f06e13f444\",\"tags\":[[\"locktime\",\"1781734544\"],[\"refund\",\"02afab98512efa342052a068ac3abd3e7b9281705dd0e4b322d1469817f063416d\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "028553d7190f007147658238bb89bda449d5aebadd9439f9a19addd6d625bc15e6",
      "witness": "{\"signatures\":[\"74c7c86ade7190cd3665751e953ae0f3488b6355f396ca91fc40cf6c00bc07343d33f8676de298b1f1c973e73a574681c0ad6c0f8ff5c06196c2fe70320464f0\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "032fcdd4b7626c0f0c3c6c914befc746ab989bbeb8a68bf80bc703edf8f4c817aa"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c5564ee85e29b392258c97c62068bb4ffcdcd8d527db366be5a62cf8445d5303"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_locktime_before_expiry_primary_only label=primary_before_locktime\n%s", payload2)
	proofs2 := decodeProofsFromPayload(t, payload2)
	err = validateProofsWithLocalValidators(proofs2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_locktime_after_expiry_primary_still_works(t *testing.T) {
	setCompatUnixTime(t, 1781727345)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"a1b05e4e81ba5dada29064bd8b16ebc442420e75e7cb088e1bf0107d15995b29\",\"data\":\"030f4aa90fe965ca79ba18a9e7342f64fd2888200ca9d8bfb3851bcf49445c3c5c\",\"tags\":[[\"locktime\",\"1781727344\"],[\"refund\",\"03c4134adcc0ee7fa21269b21133599038efbbba60069528f2dceb7ab269a91174\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "025622f872a2c213c49afdd4b2faa9bfcf005a3f4021408025e1a1cb67d24ef716",
      "witness": "{\"signatures\":[\"3a3a92371fc7b2fcef4c00fa46518bfe99c000ae66f0b6299de5664900c72534f965060c133a355c00d47fb7aeac8b4584cebb5f453a286df761107639376cec\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"3b23e243897b064a4c2d4e44a68e819c6dc413925144d517085162290cc92b5d\",\"data\":\"030f4aa90fe965ca79ba18a9e7342f64fd2888200ca9d8bfb3851bcf49445c3c5c\",\"tags\":[[\"locktime\",\"1781727344\"],[\"refund\",\"03c4134adcc0ee7fa21269b21133599038efbbba60069528f2dceb7ab269a91174\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03aa100f86132b17c4416b77c84aa35ec2950745013c5952b51b289ece4dcd5175",
      "witness": "{\"signatures\":[\"3a7fe7d8fc9081829b793899b04b8b178f4a737e53cd5d24abb936544e7b5712aaf1e418de0d52a08ff9db6298c11afad58c4bb66d032b57549e8cc298a5e905\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02ddee3ae949e0914a9b63da9d1f4ad1b8bb5bdde47ad3956d1a62f31709ff942e"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d7d3527ce4ccef70786c1f2944d940f19d5954833b63dea9b0ce1822deb799de"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_locktime_after_expiry_primary_still_works label=primary_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_locktime_after_expiry_no_refund_anyone_can_spend(t *testing.T) {
	setCompatUnixTime(t, 1781727346)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"21ca017f2b385b7355346c7d80fa1c36548e0d460081dde9549fee01a7244d03\",\"data\":\"032b93fc35c9e5a90484d6914c1bbc645dc9e9178af0487c0bd94ceb07366f3a7b\",\"tags\":[[\"locktime\",\"1781727345\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02b933602bc210edd5df8ef724a3010680d5ed074a8dd675e318cf7595dfe16af5"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f8ab8ea3b0bbf3f82413715861b00fb842c757a05b037d9db0f643898dde043c\",\"data\":\"032b93fc35c9e5a90484d6914c1bbc645dc9e9178af0487c0bd94ceb07366f3a7b\",\"tags\":[[\"locktime\",\"1781727345\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "033f18a7c7ae7c769a2ff7d54a3220eaba0b8b162defbce2adb65c4653b22d9b26"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c8effcbcf9869ba52c42ff7c7fe3d6c41974ce44d64dc7b4fb3eb07df1f146d4"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "031d726cdc2b1bf15640e153111c9a805635efbac0db0c977b7b87af0d8f6fcf5f"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_locktime_after_expiry_no_refund_anyone_can_spend label=anyone_can_spend_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_multisig_locktime_primary_still_works(t *testing.T) {
	setCompatUnixTime(t, 1781730846)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b0058af71dbef822fe92eeb33e72c8895c6302cfd6e3ee526ba32c8f11c7f9aa\",\"data\":\"0279d56b70eac71bc05aaac1e8ad623262a7a98390afc9f40e926dd88288eff4e9\",\"tags\":[[\"pubkeys\",\"03a24f7e9ac00c4ae2e2afd0bf88d92de6fd11913f36d99e4dc8fce2ca05767595\",\"02d87a49bdcd746d4a146eeb797babdee13b540fa3830f35074b666a7707274bcc\"],[\"locktime\",\"1781730845\"],[\"n_sigs\",\"2\"],[\"refund\",\"02c07c12dbdb3a747631f58837aae79610ce72c29456c22bec32a9daa46e411df8\",\"022c0c8fbfae2f98f6d1df87b494903296706225a2fa3060fcfdae87f09cf45851\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02500e7953584e711d188e5ce7a6e4e81c83fe95264949b1c84648f3b4462f2b9c",
      "witness": "{\"signatures\":[\"7deb10e044f83358432a050d7925796091d1653b2354d78507041d37f2c70e37180ae95793f5f403471137446bdc7cff56bd6283e0476326a0d5bb5f6908d602\",\"0ea5f3f661f3ba590b90a296d95ddd9b95064c96a541ba85c751659879b58f4994c3713eea893a971ae7af8e4743d9b1f6908a28bb6d0517acc222d79e61d41e\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"d0ca85e927ca4d9475b9cdb525b2df2ffd73463fae269bfe54f3e283c79e9905\",\"data\":\"0279d56b70eac71bc05aaac1e8ad623262a7a98390afc9f40e926dd88288eff4e9\",\"tags\":[[\"pubkeys\",\"03a24f7e9ac00c4ae2e2afd0bf88d92de6fd11913f36d99e4dc8fce2ca05767595\",\"02d87a49bdcd746d4a146eeb797babdee13b540fa3830f35074b666a7707274bcc\"],[\"locktime\",\"1781730845\"],[\"n_sigs\",\"2\"],[\"refund\",\"02c07c12dbdb3a747631f58837aae79610ce72c29456c22bec32a9daa46e411df8\",\"022c0c8fbfae2f98f6d1df87b494903296706225a2fa3060fcfdae87f09cf45851\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "027bfe2e8183a1fa719dd010eb3273c8137bed09eb66d249d5dac885d5a5a10562",
      "witness": "{\"signatures\":[\"efeffad5e2d28d23c3e974a2b0a78ca2b70a163f677a7c86375082b237d937c10addc39a067f5a459854d92842165c3443b98ee70db5a382b674e8dc523181e5\",\"fc54fb11841311f1adeec36672953deb1b2de7ea7a2dde1353b9204027205415da4112c7b0a904697f5610006b14e76cd9e9ba8ff928cba709d128a1af34b65a\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03074b5eb10ee56a43c6168067ae18b304a3064aaa28d2a87e1f777a99fa0ba7dc"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03b1d072144518c1a854e92965cde96047f861f8be90d6df488034c9684d3850fd"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_multisig_locktime_primary_still_works label=multisig_primary_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_wrong_signer_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b1e3b6a27794d510b2c2330f7a627dcda9df4556a5f7c50d80ef7e2af16e0946\",\"data\":\"02e72314f6a4da38bf950eb94498cb903e9c4c0224e022403b22cf2b8938db602f\"}]",
      "C": "02e86fd8c53c09cd8e6079b9dd1d04bb16df1f5160ed7f72e829f95f5befcefe1c",
      "witness": "{\"signatures\":[\"68fd44f3235721d6f70571cc23586d50d83bc805d4715e31055535a7ea45a4f8b8b35f344d2705039574d9a3dd547485dfa35269cb7ff097d35147dc877d9347\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"6087f926a0d7bbe06ca53c51230af3dd1da622874e76e1e0e6ae1e3f0da035ad\",\"data\":\"02e72314f6a4da38bf950eb94498cb903e9c4c0224e022403b22cf2b8938db602f\"}]",
      "C": "023bbe22d446620dc3e2f501905f27901597208fb1fb39cc392e879824632e476d",
      "witness": "{\"signatures\":[\"312776536717b4995215731f4adb42c4de1fdcb1f5d3dd29453137751cb69715793fdae22ab023ef4bfc09b70b17b9003ef1117a8e6f1b08ca1098327cf91017\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0226e4a2086c007a15d77204f1ee7a49ceba89aa9a51722976f6aed8f80b493cd6"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02d05915ac5c43cfcdf0f535636c0f6363d89b94c18dafdb64e85deda6d1aab097"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_wrong_signer_fails label=wrong_signer\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_p2pk_duplicate_signatures_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"86b946cc0be657701dd16cd04b4c80234c1ce5ade72390504892a2f1f1401a01\",\"data\":\"02fe8183699c5ce1580e1f0ba3dfffd32d6307abc00fa147755140710213b5afdd\",\"tags\":[[\"pubkeys\",\"026d3d74421ba4ce524d272c4f745f5f46079566ea8529d9e7ba3b65d7c7ed3679\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03bd98178ee942b49d4b0f7926314f75846c8fdcfe3809cb28ce339754d42ae3cb",
      "witness": "{\"signatures\":[\"278ed5f06897bbfd6afbe80c6dc15151bc1ae1011b0ce2dc48fbaab91a3e08258a4b6c43224f7b2debb335767866bd731afe89218bfbc159963997e051f89fdd\",\"05637c58430ffbd98ef4e5368d56e2cfba3c715d84a40ba8619d77da8b92f6880871916720a6b30c7ccf68235ed60695d45874a40c20ded741028a7915c5de0a\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"6c0fc9bbf3c8444ee7735ca48e04b58300c5ed67e48b70763b12779ee7570949\",\"data\":\"02fe8183699c5ce1580e1f0ba3dfffd32d6307abc00fa147755140710213b5afdd\",\"tags\":[[\"pubkeys\",\"026d3d74421ba4ce524d272c4f745f5f46079566ea8529d9e7ba3b65d7c7ed3679\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02f8d242f40c07b9e08944a6c42be39030d0bc1d4de9c798cabb4ba2de99267946",
      "witness": "{\"signatures\":[\"0af460b91312b0fcb8e64818f0ec5148736288a4167abe2230f8622701475c28fd969513f113fc3b1b043c10a29e5fb8810342c2ec2ac6f32c74ca1531c42a53\",\"00581b06d6ce4e62ee658d578f2a92fa2dc43e8910331be952752f894ae47ce87533e53b98992e935d3465b54ab9dd577f236ff0fb0067e28e3652b9d92b3559\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03445a429f695b3579bc8c972ab68e02a6ebf6d5ba95070172e47aafb74eb35561"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "023286196a6f45b1cd41a4b5248fa102c344b96b9de4e65822a363352284984a10"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_duplicate_signatures_fail label=duplicate_signatures\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_htlc_preimage_only_no_pubkeys_succeeds(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"c75e808e88fe1bb2fb84f56a8362cdb45ac91d15e378c647d54c309af2f2970e\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\"}]",
      "C": "0209a2f1b35135c8eeb159c10307634bd43317d4e9988d462dffadf49e4b2590fe",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"5c6ae8608fb7ec52f8b4906b7b74600c2ce5b381747e77289678d7b45f5cb503\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\"}]",
      "C": "02f883885c4c368c03dfd0371aad26ea724b61f29ef3fb19c1dbd1d77747b690d0",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0386044335bb707cdf608d78230ba88821f37ba459f28533cb3e24a6d4482578d7"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03a6230398419c5431d80850e67ff033da81b8c380f4cbb2e9b2fb51ecd1787051"
    }
  ]
}
`)
	t.Logf("scenario=htlc_preimage_only_no_pubkeys_succeeds label=HTLC_preimage_only_without_pubkeys\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_preimage_only_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"241bb861cd18c4a0e51806131a92687b15a6f68be68d745247b2a8e09344a888\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03662218cb68968b612659914e01b41e09eeeeb4fe94b4b82e58a8d8cec873d504\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03e45fb7068cb06446891f6af4f71d021595d10a4e05e77d3aebcd89dba6c20166",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"931e3ccb4bd8a0d4b8d9a55298e2b6b6a4d6e3d0dcf9fe4786182b97f2593a8e\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03662218cb68968b612659914e01b41e09eeeeb4fe94b4b82e58a8d8cec873d504\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "031f14085f598d1c6570f84931c31ba13ca91fc2c26236f2807013f1b78e1e2a6f",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "037b92a3ea9e9e1c55aed938e818e7feb86790f3953ded1f08419bb8c9de8530d6"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02f30c88b17eda4d009112bbc11c08b948f119259626fca034d0053107611b751d"
    }
  ]
}
`)
	t.Logf("scenario=htlc_preimage_only_fails label=HTLC_preimage_only\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_htlc_signature_only_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b4ee33d05d0e476e1de66b048344abae57cbd9044be7224306b3e56634fbaba3\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03409fbdf017666ee4c165f87c44cb724b001d4865ae6d2784771f2d82d6944480\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02cda1feafbd0765af4242d12abcd2fbc03c876320056de90b2e818f908ac64842",
      "witness": "{\"signatures\":[\"5abf57496654a08a5ba2d39984b03d22c480e866a8327109ce3dc1ae34bad8aea149eb1379d2d2468076ed3172853634a939f93196f37419ef267abbfb700842\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"d840d035de6d71b4d0ba1b424e1cf4ec4dbe17ded59ff503e93ce9f9a08dd508\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03409fbdf017666ee4c165f87c44cb724b001d4865ae6d2784771f2d82d6944480\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "038656eb39e52eebc93b4f7f4235fe64a6086c7b254df54c693f795c647d7c0bbd",
      "witness": "{\"signatures\":[\"d2dc110e4414a2926c325a0d933e2d8e45ab3cac5c461af675c52cc7172b61959068b7256f02d756aa1bf6383adba67e84dff4c555ec403943d22a9a25b5b73d\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c8dc6f48ecc436e8b3a2f0c3bcadbb4403f9ccea8030f757e3f2c79dc3514b25"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c7293f0bcf8b5ae818e366b0e2d4fb40e2ef3293fbe98f33a079ce1e88640089"
    }
  ]
}
`)
	t.Logf("scenario=htlc_signature_only_fails label=HTLC_signature_only\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrInvalidPreimage)
}

func TestCompatibility_htlc_swap_preimage_and_signature_succeeds(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"ad67180ab0603a530bfc7190c40ba2f588d676a9983531da1d5db27c9efd13e3\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02d836aac21821ec94df177ab6093936ec8716840e5b2a319846a5d24cc5dc89d8\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02eb21b80dc0a93977e62b4292ac4feac71ed6fc1e17244a8b330d2e186e93e83f",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"563a9d999c17d3c81dfcb5d9949be156c69feb34005b6c2840f098d317cfc096f0e18aebf4c722002813495f01d5b0c138f37808fa3f4b144443be1c815e2156\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"27b319864ac33548387e98978e8a369aa2bf73e0f3f29533e7bc18b0fba87866\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02d836aac21821ec94df177ab6093936ec8716840e5b2a319846a5d24cc5dc89d8\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0205e7f41a9fede572928b50af41495f7dd4e0bdc51b73a701810ca7ca7578d213",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"63c781f976a9a0a7483b4e935576f99cc5b545b8fe03d3809c339aaab0252733f0a980b3932006ddad6378431e1c39d3c6d5cb999f9ca39fef15802d1b7a57c0\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "028d2575454a0196e2d51852f7546737ef88966160edfed8c37b37cf27057b50f0"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "023bc32d530c47f7f75ce22b736c28f5fa1e2945c7b3a39ff8256dcb2a1bae0723"
    }
  ]
}
`)
	t.Logf("scenario=htlc_swap_preimage_and_signature_succeeds label=HTLC_valid_spend\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_wrong_preimage_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"e9d53406ca0d43ad0d6e6711434f442a2c5af304c30ea3d0b8bd648fe1c0e812\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0333f8fb0944ddb7c7f3769cbf1d70f10e68d349e052f6e95144e6b3b1866b5b88\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03e62261cf13df606853b3c6d424f407207244c86e875c45a574eb4c4c374cdb03",
      "witness": "{\"preimage\":\"this_is_the_wrong_preimage\",\"signatures\":[\"8b17c95e765c974a726f38da492453d0a0476db5d346feb660028c5475819d3842994240cac710fcd2bb0d2c75b8ce06a284d414cdba1f08f5c162320def0419\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"f989f7f79582423feffa334afaba1de54ac149979b480d39e7b6ab77ccf66f98\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0333f8fb0944ddb7c7f3769cbf1d70f10e68d349e052f6e95144e6b3b1866b5b88\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "030214a956cd7a9c870226eb17c830334da72bd923f1a677c6aa40e7206b47107a",
      "witness": "{\"preimage\":\"this_is_the_wrong_preimage\",\"signatures\":[\"f4c52c1f691768d7cf518cd766baa38c03113556d6a40b5dce5cc2a112f6a92dfbb95dc1077f1de9175646ccecffae2d0d36e90caf6fde301ecca098ede25043\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0318d737e2690073bfb77d094b9e50a0a95e78d4025f79b2a6973dac1c7845dad7"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02db960c51a3c4231535a3e5bca21f7630b96ea6aa8f3375df11d6a1b5f13bc291"
    }
  ]
}
`)
	t.Logf("scenario=htlc_wrong_preimage_fails label=wrong_HTLC_preimage\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrInvalidHexPreimage)
}

func TestCompatibility_htlc_locktime_after_expiry_refund_succeeds(t *testing.T) {
	setCompatUnixTime(t, 1781729948)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"49326881cc767dda545c2c90011693209fa9d83b4f9db7c9152026f651b6a767\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02c7e6d6ba5be9e2ef2aa14c4eed2bc0ef628f2a0053d5883294f8c4d88b8f545a\"],[\"locktime\",\"1781729947\"],[\"refund\",\"02b5b34ae63fbd25f92dce9d8b4c081292c9e23983dd581a1855889443fa8ac771\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0339a1fabcafbceb6c952620267c272bb464c872dd3004f5c7b333d3a97e3563ef",
      "witness": "{\"preimage\":\"\",\"signatures\":[\"e9a3ce7aee137ba155d6966ba0ed15b3e7554b19befe4d85da81aece08f1a31e56e379db76009256404dffcff624362d1d5ea65fe00d23daa621e1dd54e77c31\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"ac2e2566f2056839d70eba30266f7dca19994ab2c74a0308ddfa4c8accd0fbb6\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02c7e6d6ba5be9e2ef2aa14c4eed2bc0ef628f2a0053d5883294f8c4d88b8f545a\"],[\"locktime\",\"1781729947\"],[\"refund\",\"02b5b34ae63fbd25f92dce9d8b4c081292c9e23983dd581a1855889443fa8ac771\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02c0fa7a5f67de32d1a0be11e966fcf77c479214cdec06473c8130b8fd4ca83bec",
      "witness": "{\"preimage\":\"\",\"signatures\":[\"dcc7cbc670f777243a058d9334298d67b164fdce3154294ffa305bf6e455a42c9573843365e27f1aeee27fcc1e3f545d64f6a4309ec1a21a90aab61b24eba047\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "032eec278dd66d5955337b6744c5961b58f8f067286023f7b5dfc55f02a99d04ab"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "035de85787b756b3982f5b7e78643971b11c3b08a839a6dfefac23a06a69e63827"
    }
  ]
}
`)
	t.Logf("scenario=htlc_locktime_after_expiry_refund_succeeds label=HTLC_refund_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_multisig_2of3(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"a19b075f4cc2d1e9d8e8ccd6722228ee0a8931a20da4f20f5d6fe2d1250b8d1b\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0208b4ff71800339677991916dab228845e27edd269c77e6aeabe4ed92871bb122\",\"0302a269d2da53cd3585e676984615146a60cd2e98547a4dfb89d035091578ab2a\",\"025b8d335f884def4da987ac40bc2d2b9a37e1a763823422fdb56ce560c2710c0d\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "025d2b0908ce56e2c3a114d6618dc2f7dadb7061029a239d890715bff82aa68e57",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"88d68d8ffe7ac91480006a0231492e7971d1219c4db9d02d9096151217885e77af7c8072f6af8a1de7d6a94ff81ee7798622cd26ebf6613ff4e736e9b18368b2\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"5d1f08bb5655946ed8d519a8a337fca17dfe872e3fbbce6fb978174c3f397637\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0208b4ff71800339677991916dab228845e27edd269c77e6aeabe4ed92871bb122\",\"0302a269d2da53cd3585e676984615146a60cd2e98547a4dfb89d035091578ab2a\",\"025b8d335f884def4da987ac40bc2d2b9a37e1a763823422fdb56ce560c2710c0d\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "038effbd57c6b62aa0dbcf51340e7d105d95787f8145910d2b31c3e8cf16f2f312",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"4a304896a849db0f39c61e104580f2dfdc1c41bea87e7be724a3843af053518c3fccf015bb5787fd93187fbc3f980a3cd07ef914e354282e230afdbcacf26af3\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "027fe5e35447eec99b63eeb0d2fb8892aadff9c7a4461adda3466174ba6218f4a4"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "031ad6bc910642bdb39008d57e7a0e58b555c7952f120f9d7f40b67bff30016669"
    }
  ]
}
`)
	t.Logf("scenario=htlc_multisig_2of3 label=HTLC_1_of_3\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"a19b075f4cc2d1e9d8e8ccd6722228ee0a8931a20da4f20f5d6fe2d1250b8d1b\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0208b4ff71800339677991916dab228845e27edd269c77e6aeabe4ed92871bb122\",\"0302a269d2da53cd3585e676984615146a60cd2e98547a4dfb89d035091578ab2a\",\"025b8d335f884def4da987ac40bc2d2b9a37e1a763823422fdb56ce560c2710c0d\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "025d2b0908ce56e2c3a114d6618dc2f7dadb7061029a239d890715bff82aa68e57",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"5a91c3e3afe0136fd914fef48c3e0eaccf0c6c4ee98d8ddc640dc049d5ddebf7044f27a1c9eb91103f5de941f9d15b75414e53f935ef8c82a85919b6108e6398\",\"5539cae444b074905f656c3bad9ffa5e92c753b08930e886687cd9ebe1d7b99c488959751b032d6107ca1a952cf9aae3fd8b039ca95b083d07225180d1f67a0f\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"5d1f08bb5655946ed8d519a8a337fca17dfe872e3fbbce6fb978174c3f397637\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0208b4ff71800339677991916dab228845e27edd269c77e6aeabe4ed92871bb122\",\"0302a269d2da53cd3585e676984615146a60cd2e98547a4dfb89d035091578ab2a\",\"025b8d335f884def4da987ac40bc2d2b9a37e1a763823422fdb56ce560c2710c0d\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "038effbd57c6b62aa0dbcf51340e7d105d95787f8145910d2b31c3e8cf16f2f312",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"a12e5cad6f34b14f51cb0f7753f9e89e40747fd8745839f8df68fe2fa349ac514c351989feba40c09772a84c6d81b0dc8a4ede1dca451ac2ebb6d23128747fc7\",\"a0b70fe70511c09f966334be675426bc948e583a05e705cadd0b508ea9fcf541881d1e23793383b2e74951add43f74ee4ea386b8fb3c20e456fc7a6d5ea244de\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "027fe5e35447eec99b63eeb0d2fb8892aadff9c7a4461adda3466174ba6218f4a4"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "031ad6bc910642bdb39008d57e7a0e58b555c7952f120f9d7f40b67bff30016669"
    }
  ]
}
`)
	t.Logf("scenario=htlc_multisig_2of3 label=HTLC_2_of_3\n%s", payload2)
	proofs2 := decodeProofsFromPayload(t, payload2)
	err = validateProofsWithLocalValidators(proofs2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_receiver_path_after_locktime(t *testing.T) {
	setCompatUnixTime(t, 1781729948)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"a2b4f3f6bd391e7345a348ee3aa8aaccbcdd8a5d0c3f4ac5c3f8b1c3da6f77ac\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0272857fd8a6ef5a8b079d29b3a6fcbb745f11331c43c971bbedda1e1d340f1ec1\"],[\"locktime\",\"1781729947\"],[\"refund\",\"0285cdb63410cd2b870d5f8d4fffd282ce1928a064616a54d3574b500cbbd8bcf2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02e202565faef0c0166246622074074a2b8faec9a0c7bc5812d00db195a6338064",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"e28bbe94d2d3a21f31987691b866fae07e819d5a326334bb56f69c620c4e1da3167105055a29c7fcab82de09d49ae84c498e7a2716212b38de43d0e3c10fb5c4\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b9f6b27b8826d8c590d88c3f7649a4417d48df1cb6cf59a754177e420bcfcc14\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0272857fd8a6ef5a8b079d29b3a6fcbb745f11331c43c971bbedda1e1d340f1ec1\"],[\"locktime\",\"1781729947\"],[\"refund\",\"0285cdb63410cd2b870d5f8d4fffd282ce1928a064616a54d3574b500cbbd8bcf2\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03450d144d0592a9025f8dbc21dc60cbca47a14819e503bbfc7823156fd63a0638",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"99e1157c821611d31d8bc29931e06e9118b2c8123c36859be7d6e81f4e1ff018651d193183f9acb5010c43138e36fabf80f8a9c58c209ea39df089e025fc5642\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0392b6892a40433edb6085f3ca4ea608df95c8a0bff683e840eecc2b02452bd601"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "037c9235c67658bc6564d3be8067c41d80bdcfa325dd9df4378a9afc4d1729b8b8"
    }
  ]
}
`)
	t.Logf("scenario=htlc_receiver_path_after_locktime label=HTLC_receiver_path_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_p2pk_unsigned_fails(t *testing.T) {
	payload := []byte(`{
  "quote": "paV3ur3q6w-oWIMlKxP17FVJbAmxnyC1k6M0ghXX",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"407e26770c0889a558a957824681e45870550e13fbfe2724594f64e58870f719\",\"data\":\"038a696fdb339ce5179adaea1d17bc2da45bc8086fdf214d7f315a9ae1acd58670\"}]",
      "C": "03cd7601903e9eda823b97f7102ab14ab191691dbaf679a2d6688acf19dc2c6edb"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"92fea47e26cd7db5081c615dab3cb35cacfe2742f633343ab484700e74fd770b\",\"data\":\"038a696fdb339ce5179adaea1d17bc2da45bc8086fdf214d7f315a9ae1acd58670\"}]",
      "C": "023c4b4e12c83a0f80c0a0ac4c39a8886682a04da6d8a5ecdcea4a6a3e7f44cd5a"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_unsigned_fails label=melt_P2PK_unsigned\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_melt_p2pk_signed_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "yrEtkkW3n4c-w6ybDyGJgHPkHraUiYJvyVBgiP_m",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"879b84468b63c8f2ed989b10b088bb6a7f3fd1fe90aa59c148fd66280d89e4d7\",\"data\":\"03e7174902bbe74572563efcc1043f5cc227d650dafe010188494587bc49499af4\"}]",
      "C": "039aaa14e8632468521f91dbc64b5193606010b8895e3217fe0882113dfd8bffbd",
      "witness": "{\"signatures\":[\"454ba85c3c34673473d9161d15229d32c5017e028e0cb6abc3ac9fa22ae28203e5ddb027e87b10656f97dab301c9a5edd438eb38ecb7ac8ae59bbde5d315325b\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"dc36470f5d1e8e852ba343ca0edb589996228cc064333da98f923b824af0358d\",\"data\":\"03e7174902bbe74572563efcc1043f5cc227d650dafe010188494587bc49499af4\"}]",
      "C": "024f997e517ad975cedf68c7c45f1308562c2e746df62e0a4326d1bdd86683433b",
      "witness": "{\"signatures\":[\"81b234a5825ba9d5b76f34d9262b8881a06d5ba784868881adb6800e2054c71fa12a6762d5536f0c7f117b500ace84f5e39b956cf2602aae709bb091c355c80b\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_signed_succeeds label=melt_P2PK_signed\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_htlc_preimage_only_no_pubkeys_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "rDtihu27uvcLbL42BA2ah3RjdKTvHRya7ZEiIUyb",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"5d31e982a84a388a82d96bdc79134391c6ba85b057be5b305a37346710c989b0\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\"}]",
      "C": "020214b457803af1f84df725348edf6e3260fe2ed0860ba54b57e97e66c6cb4d6a",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"dafc0f223369c9d763bd520d529abbe8ce4f24f17868d5a7806e2364bbd04e14\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\"}]",
      "C": "02ca6cfcf30b314798cddbe2428e049f6eef391c60ffcb5b1b6e97ea63608e640e",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_preimage_only_no_pubkeys_succeeds label=melt_HTLC_preimage_only_without_pubkeys\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_htlc_preimage_only_fails(t *testing.T) {
	payload := []byte(`{
  "quote": "Jx--4Ceq-OWjthsZqpmH0GZ-JrGPjs2k7lFUe-ki",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"6d5d8971f2c9ab3a0ddaee49f2fb01da6b7ed22082d9d5752f34514a6bcb5d4a\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03d08996e7ef5067e8015620aacd8465cbc698caccad6b1eec210996b54f8028c3\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02fe97423d08585803d18a8b43bc20e35a116f3313749b215909678948ff8f23e2",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"0b195df2151d659b405cdebba193f7d2bbc20b364d1ad2a1998c431959a955ce\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03d08996e7ef5067e8015620aacd8465cbc698caccad6b1eec210996b54f8028c3\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03bd51d462ab0a87c4a48e24732ac349fd297afc13cd458c86bf6f1304197ae274",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_preimage_only_fails label=melt_HTLC_preimage_only\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_melt_htlc_signature_only_fails(t *testing.T) {
	payload := []byte(`{
  "quote": "qVb5uKgiVFpevx3KSvuyRcwXPJYFzOqnFjnR_ka3",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"f4a68b90a19d092eeebee054a35aae177eaec3bf0b9c3344090a3b41c8e7b100\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"038a8a894628535baae2fa4241ee7780776a9df595d23a42e838eb14ca5ca6123c\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "031bc38480d7ffdf20dbc58a2cb14cd717435ea3f054f5351bd75e6609c8468a8e",
      "witness": "{\"signatures\":[\"1e1db8da17201b027e3f52e50f97982ed71a4447abf3402287c264753b763dadda2e2774ddd6cf661a5c47ffcab6eab466bc533957ed3dadc4cc9303902ca48d\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"9516313c6438e11f5e49f74fc6db26ea30efd37f4242af43e387bb0cb7803ad9\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"038a8a894628535baae2fa4241ee7780776a9df595d23a42e838eb14ca5ca6123c\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03f6dbe0db084fd0d600d8a3537b7986074e72ef979b6bbad0c8c1495a8971781c",
      "witness": "{\"signatures\":[\"a72171150fedafb6fba495e5ce3ab55c6a0e5080fbf2ae8fce7823fcd9a02219ea897b0a16a17082f5512341cd69a0f480dd91030d8d16b3377f308bb23179d0\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_signature_only_fails label=melt_HTLC_signature_only\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrInvalidPreimage)
}

func TestCompatibility_melt_htlc_preimage_and_signature_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "AdDAmAcm2TEuNJW6oa7bA0Ldf3ba4f3XtjW9d0M4",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"982d4ed1d5c94e92ce5c2f1e87f5ff7f8739c0895a399c4a0198b4bb687bf0f3\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0225f3a0c96ce568d37441742410ebd0c16633a35f6fbcf69e35349daa4c5802ac\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0234a5e9e501ca281feccc17139717a2cc83fc9ae805da13676efe6b490786fd6e",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"61176c61c9529a5b59e0f2ae9a60f7e533fa835f75f2fd3ee448e270440a9d17fe5b06c02723924074a85ddf8fb4a0d866af78bee5e00c29916e92aedbefd466\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"82d312ff6a00ef608b6002c23515de0bef3e68e58c729e290fd4a9e840b400d2\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0225f3a0c96ce568d37441742410ebd0c16633a35f6fbcf69e35349daa4c5802ac\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0369c41e97b7f9a15e430cff2c3a4df51ddd8ef3bd67b944f391c0b9aeb2b39e43",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"ed48339b9baf5864eeb9c4ff4926af492b04502259a28c7928fe5158294b0a1f5921147d7574cae3ca8650de8f0d1299b25e725e9eb09750ed45b3774944825d\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_preimage_and_signature_succeeds label=melt_HTLC_valid\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_p2pk_post_locktime_anyone_can_spend(t *testing.T) {
	setCompatUnixTime(t, 1781727359)
	payload := []byte(`{
  "quote": "QVTBsCZEKpelA20IzxOGA4v4my7OyVvsOEpEAYJa",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"47a3c2970c6ac6db4081ba14ee3ec79e2086490be70a2323153f51b9079dcc1e\",\"data\":\"03706095b6e4236cc4fb8895f70c786f2adbdb7825abc70adf302856e3eaa340f2\",\"tags\":[[\"locktime\",\"1781727358\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0213f206e8b58663207018208902dff179f25d91fc8313619c9404bcc09c9579aa",
      "witness": "{\"signatures\":[\"54675d1e7dfd61fdaa41f423f7fa67e86bb482c3b7938f8cbf0c9a090316fb944fb426d9557b23d5fc5cd8d0b65d7449a9871dcefaace609ce3b11805ab79fd9\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"6e631a1ead601b59932bdd578f69aea872c9463f8d141e411b0965c90c8ecb09\",\"data\":\"03706095b6e4236cc4fb8895f70c786f2adbdb7825abc70adf302856e3eaa340f2\",\"tags\":[[\"locktime\",\"1781727358\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03fab2891978cfd2298dcb3362ae45894d4677ceff1cbc9a224a15757726a5eb5a",
      "witness": "{\"signatures\":[\"51438ba0010dce00f80c56380a73f69eea3e432a54665b0d2ebc25a4f4830c4cecae1572983885ef729e78a4717fa22aa690856f5d94d722f5cd1753cb0368b8\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_post_locktime_anyone_can_spend label=melt_anyone_can_spend_after_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_p2pk_before_locktime_wrong_key_fails(t *testing.T) {
	setCompatUnixTime(t, 1813266958)
	payload := []byte(`{
  "quote": "LvOsRfgCp2OWwiECuBd_j-JGXV5BqproSJkd3UFq",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"a107687bd545c787cfa983723f5873b7af9b502e718d1f58c776950fcd936a06\",\"data\":\"03209347d1bf6c264170dd14dc43cacb9332f8036d7fc9a70550409e66e9e98f1d\",\"tags\":[[\"locktime\",\"1813266959\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03228fc280d902466de09cbdd98bcea4c6463c7aa4c93c30eb5b9a30757cceefa4",
      "witness": "{\"signatures\":[\"9b56f434bfffb282fb8b21e9e1059bd72a340feb43bd6fcc7283a77f4679047da5ae30629f6f83e914e3bb52c31f48b35c8df57d7808f57edfaaf35a5fb542d9\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"fca2b68506f1466d85481c30a45529f23068da00687b9e138c7caa808e95a38e\",\"data\":\"03209347d1bf6c264170dd14dc43cacb9332f8036d7fc9a70550409e66e9e98f1d\",\"tags\":[[\"locktime\",\"1813266959\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "0341d278fbf120c0381ca6f707001367a07cef5a564e82940a3271f125a7c3b48c",
      "witness": "{\"signatures\":[\"5f10ae9539e8b88481ba66f6612f2127597a155e61cc1032991301bde7186c297620a824c2faee6e4ba58d59dfdc167450df2883950bd90ffa0ea40749065f67\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_before_locktime_wrong_key_fails label=melt_P2PK_wrong_key_before_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, ErrNoValidSignatures)
}

func TestCompatibility_melt_p2pk_before_locktime_correct_key_succeeds(t *testing.T) {
	setCompatUnixTime(t, 1813266958)
	payload := []byte(`{
  "quote": "mSMWjcCMjNoxNe7wVU9bbTZMwviE5grQViW57xZu",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"3f3b2b8c8abf30671f1babfb733fd7e38170545a9ae885a22844775271b58f63\",\"data\":\"03c933f304db7c81453677cb3b1b7765c5cb58a1d91e55896cdc8b423286e0c5c9\",\"tags\":[[\"locktime\",\"1813266959\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "03430201884f0bce2e4ea2e21f080130ca4a78579d99ae79d2b64d609af9f55d6b",
      "witness": "{\"signatures\":[\"96280720e9e1fa6c104fbac2d1cb94f126866be9ba7fab5cb855294ca2bb921bb7ab8b6e4a31875483e36460b9d90a884156f10a6891ab70e63d6f30804787db\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"42c9d55f9162e37f80695afec30b50e0b15977279ade5ea0d5c9b2f803b0a27d\",\"data\":\"03c933f304db7c81453677cb3b1b7765c5cb58a1d91e55896cdc8b423286e0c5c9\",\"tags\":[[\"locktime\",\"1813266959\"],[\"sigflag\",\"SIG_INPUTS\"]]}]",
      "C": "02d85c657cdd7bb0f67eacc5bd290188fc6d63511558b88770cfec2cde44564b73",
      "witness": "{\"signatures\":[\"bbe7ebc72b90b5b32ae797b3f2df5ad166233feb4b894ec4fc8b85758fe8adc63741b21545bd4ff3bba14d51d56031920d318f1d46643016fcfa5c6b54cdef93\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_before_locktime_correct_key_succeeds label=melt_P2PK_correct_key_before_locktime\n%s", payload)
	proofs1 := decodeProofsFromPayload(t, payload)
	err := validateProofsWithLocalValidators(proofs1)
	assertCompatError(t, err, nil)
}
