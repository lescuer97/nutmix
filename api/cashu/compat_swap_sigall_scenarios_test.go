package cashu

import "testing"

func TestCompatibility_p2pk_sigall_requires_transaction_signature(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0b84ffccbc210c94faaf1f9833862f8e9d463fc9c4900cf652cc6d4a7bac38d0\",\"data\":\"02b8aa2d8a880fbd299ea68d851b7fecdeac6838cc6f177662a4aebc5a74e97c4d\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0321a4889137cd0b35371ea2c1cf1cd38b96dbf264f3cc3ca44c50a1c3a241fe30"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"ee0afdb2f89a2866c7517cc5d1ea56295788636425a2def0b3933b1e62c88226\",\"data\":\"02b8aa2d8a880fbd299ea68d851b7fecdeac6838cc6f177662a4aebc5a74e97c4d\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02f7d02aa63d9fd9e09ea41ba47eeb3448cb13bf40a9bbfaf2e63012531dd68d7c"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c7991f6d20c0870bee631fd27b9cbae50805a2ead1778f81370f40681a5da191"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "030a5640e58375dedcc5a823cb1c1889b0607812dba2a63764c70e62dd1f60653c"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_requires_transaction_signature label=SIG_ALL_unsigned\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_p2pk_sigall_sig_inputs_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"d07ef0283f7590b43df4e6c54dc905dde623131cc70dc4d4c26cd3c6da5bbba5\",\"data\":\"03c3a351100fe61c94772fdaabf4086f178e85e95687e29692d49a0c69e5c1a4a3\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "020defd2e67ae5c1ad964cb0cb373767cc21142e8af655fb1d00afc945610d3d16",
      "witness": "{\"signatures\":[\"1d0001f1db27ec80612fb1a25d6d04f00b5b95c6b32bcd508d8142e07209b8abae4e116d94a26726d9a2047275390db88a832ed68cba24cff3cd17752eb9cafb\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"c6fbdc5fa0dc1b6dcfdafab86e946b5c554659da8b38eda542ae3c8815d71b3d\",\"data\":\"03c3a351100fe61c94772fdaabf4086f178e85e95687e29692d49a0c69e5c1a4a3\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0286b38d72ed75414b86f702d6910167d1d229b48789bba73191fa765c4b84eb1e",
      "witness": "{\"signatures\":[\"9baadd6c3829cf80de63186b2fc459b7d089b3bf073083c7dc664866a44ad0c13dbbfe08f06fb8371e794d94708a7d4e93d070636090218a09dc7d930c4839a3\"]}"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03fb50f7e8db4a7569816d65fcc74425e4604603b3a585ad504edcdce50a43865f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "031bec8cb714c91e8072b20fe31a638eab84a128a5d020f5afe144483426bc0bc8"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_sig_inputs_fail label=SIG_INPUTS_used_for_SIG_ALL\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_p2pk_sigall_multisig_2of3(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"c19d54cefdf20e2d013732c5c6fed2ef66fc502718022ebef575b523c6b55803\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "027b243e337679a9bec22544c094d9d5de187ccfa1474cd5e6dcbdadccc55f3131",
      "witness": "{\"signatures\":[\"1ae8a8e39f7f2bd7184e1ba3c30a4192a96fe4fba58fa63799e415a97903869b4882a523e7647b3147cbf4596b58ffd3495debcfcbf72d5e742462907a069b25\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b79b8fad9272aabbe6b34e4825642fa94cf3caeaca871c06357dc9e0ed8a3adb\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03dd471ba8a4fa8f18ec01264d6287337225a996b02a628512d07a12454a977bb6"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d39205d1681e9fd8ca2e977be92dd0f806908faa2b4815ad02e702e79207048f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "021e41cccaaa0760bd6245bb2c616e5eba1f9a4ad4917c17e107d7e70947ba2521"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_2of3 label=SIG_ALL_1_of_3\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"c19d54cefdf20e2d013732c5c6fed2ef66fc502718022ebef575b523c6b55803\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "027b243e337679a9bec22544c094d9d5de187ccfa1474cd5e6dcbdadccc55f3131",
      "witness": "{\"signatures\":[\"23427aa16878008e8366d21a53d4e4c8b2f54ea7deafaac5adb4679eabef3284f24577b4c91d259711037682f9cdd20d363b51286558602b659f699bf72e0cbf\",\"2f1e81e8bca404c8a9f0df3f7ee95bf6a3a636f8bfa5a4eeebe2b29f2344744f87d0b1aa1aee7819f8b40973a7256c900de3185f55c6f9226583e6f2153c0a65\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b79b8fad9272aabbe6b34e4825642fa94cf3caeaca871c06357dc9e0ed8a3adb\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03dd471ba8a4fa8f18ec01264d6287337225a996b02a628512d07a12454a977bb6"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d39205d1681e9fd8ca2e977be92dd0f806908faa2b4815ad02e702e79207048f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "021e41cccaaa0760bd6245bb2c616e5eba1f9a4ad4917c17e107d7e70947ba2521"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_2of3 label=SIG_ALL_invalid_signers\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload3 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"c19d54cefdf20e2d013732c5c6fed2ef66fc502718022ebef575b523c6b55803\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "027b243e337679a9bec22544c094d9d5de187ccfa1474cd5e6dcbdadccc55f3131",
      "witness": "{\"signatures\":[\"c2bd0821a7ddf4d94b15f6b33446a4e45b86ab77c5b899ddfa4a5afc51e43964039ade903918fd64803bf0b44f18ea0e69c4311cee625f4043608e4a52150a09\",\"5688ebbd137f8ca28587974ecd98ce901cce7a90f33c439a630627e5c29872b0c0ace814ce1d13426a4f57614abeb6aa9cf4b73edcd50a70b9df417da806bc42\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b79b8fad9272aabbe6b34e4825642fa94cf3caeaca871c06357dc9e0ed8a3adb\",\"data\":\"03ee71b61444bb7a89f7720d72fa81649a3d054023b9a547931fbf0b0156c02c77\",\"tags\":[[\"pubkeys\",\"028d3a993f0cbdd586170fd7dc698de37ac14c39898245af24176641518b746135\",\"02ffa5b2900b34e0f7c7f3eca2bec0a317c774559c606ddb4eccb4ec14d090f844\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03dd471ba8a4fa8f18ec01264d6287337225a996b02a628512d07a12454a977bb6"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d39205d1681e9fd8ca2e977be92dd0f806908faa2b4815ad02e702e79207048f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "021e41cccaaa0760bd6245bb2c616e5eba1f9a4ad4917c17e107d7e70947ba2521"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_2of3 label=SIG_ALL_valid_2_of_3\n%s", payload3)
	request3 := decodeSwapRequest(t, payload3)
	err = validateSwapRequestForCompat(request3)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_wrong_signer_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"40557eda1581481fac1e4fd07722f27fc9980761caf387ddcc2eaaf3978a952a\",\"data\":\"029a8237638808d73b361c0bc5010be4ca02e67e7210021fb7555e8ae7f215b9d3\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0319951f5cb1950966729ce123cefd8eefce723a055bd84f4fa006265aef18c985",
      "witness": "{\"signatures\":[\"ad85e57ccaa9b04e06cd7cc1b8b15477d9eb3ef584ac83b1d06f6802a5be555329a3275fad9c646d50cb952382025c1636b87494f43f0dda140a9e759ed262bc\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"79d42561e0d65086ee28ad7f91af8f7748c6d945a26d48bdf91d1b569be22f1f\",\"data\":\"029a8237638808d73b361c0bc5010be4ca02e67e7210021fb7555e8ae7f215b9d3\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03a70fae4f2c2e3e3cc3f0c8717537abf6c9f1a20f6279228c2168e2fbcdc01a46"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "039530b502696da451289a8ffb33f2788a23513affd217265628b0c7ba45cbb9a7"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d7703f2b0878922e07ccfabac3c519bc4bde49cb5efcd0ca9c03d9e70761530d"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_wrong_signer_fails label=SIG_ALL_wrong_signer\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_p2pk_sigall_duplicate_signatures_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"e71f64aae8a9a774626b43319a6a793d0873aa7e63025c22d0622880e59e9417\",\"data\":\"0358856eec4c6506c03f5eae8ea32f85e5cba49c307eaa43b0071d155fa2961c1f\",\"tags\":[[\"pubkeys\",\"037f816b11e43033b84b0585cfb53bbe7f95e330d69f07d6a04e9b5b624ba07414\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "022e9ad8ef67429b139a7559b40020a4fec8cb25c96ae22d7e6dc5a18547dba0e6",
      "witness": "{\"signatures\":[\"bf88e479f45fbfbbd5e37b95ef9b0bbee0997ca8c346ac37cbf3f7bad9fe4bb0a3f7953550efc430f6c6abc586070d36666635158c40e60a5b727a0126305ffb\",\"f435fe7a77c960dc784860fc5575316b27e4f2a175d46d96fe41ab5362a05b458af1ef4522e70cf139ee6975f7190f18bee639698ffd7f0fd66137c69351cbaf\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0bd04a3f88f2b118733436a308baf3fd1ff80886f5b5a242d7ed4dc239d39d68\",\"data\":\"0358856eec4c6506c03f5eae8ea32f85e5cba49c307eaa43b0071d155fa2961c1f\",\"tags\":[[\"pubkeys\",\"037f816b11e43033b84b0585cfb53bbe7f95e330d69f07d6a04e9b5b624ba07414\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03bbd40753984e46572e1f4e47f58aa01e790eedbde302c147d21d4b8708c31478"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c6545b40b0bfe59ae3c62160cf0ebfc13626edb3132fbca7b6c594b27a7b2b01"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "039dcffc4766685b191f9dec781b0fc2ae1ab2acb8ab03a401e65d1f4f0680423a"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_duplicate_signatures_fail label=SIG_ALL_duplicate_signatures\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_p2pk_sigall_locktime_before_expiry_primary_only(t *testing.T) {
	setCompatUnixTime(t, 1781734548)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"8f89150de799e2436b0c8822800a4759677108a5486bd67837e4a6daf87bdd23\",\"data\":\"03ee40ffd61673753ee3ce619981c9868f78e928426f4a7f9d6dc8fff32c832419\",\"tags\":[[\"locktime\",\"1781734549\"],[\"refund\",\"03f02a2a968ced0c3f00b6951e10a2c135b3a2b125e1907a4fa3bdb3bc7ce2f61f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "027ce07efb89fbe53237c571048d85fdb9ea42fa2c2669345989874c715cc6e233",
      "witness": "{\"signatures\":[\"dbdea8fac937f38fd6558687b14026e01876c079f90464e759de25b9247cff8c4e012ebbcf40c8982dee402345849f340b8038f29ce5c235250c7bd04276c8a0\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"79eec14991f8f5ba11672831ad401a87764fe0ded6696011e98d4da446a6f5cc\",\"data\":\"03ee40ffd61673753ee3ce619981c9868f78e928426f4a7f9d6dc8fff32c832419\",\"tags\":[[\"locktime\",\"1781734549\"],[\"refund\",\"03f02a2a968ced0c3f00b6951e10a2c135b3a2b125e1907a4fa3bdb3bc7ce2f61f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c8a13ebd0ed8c82a5382859143a5e3cc1161497fe07df4998cfdbcd7c1d88f32"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0216bd853d7891d815051b64850118da265e5e555eed3f573a88f3967b6ed323b5"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0242f71bf738e64a652418e1b2e8a5a5739ef735ae6175dac193ad0a07ad0c8002"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_locktime_before_expiry_primary_only label=SIG_ALL_refund_before_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"8f89150de799e2436b0c8822800a4759677108a5486bd67837e4a6daf87bdd23\",\"data\":\"03ee40ffd61673753ee3ce619981c9868f78e928426f4a7f9d6dc8fff32c832419\",\"tags\":[[\"locktime\",\"1781734549\"],[\"refund\",\"03f02a2a968ced0c3f00b6951e10a2c135b3a2b125e1907a4fa3bdb3bc7ce2f61f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "027ce07efb89fbe53237c571048d85fdb9ea42fa2c2669345989874c715cc6e233",
      "witness": "{\"signatures\":[\"6a01814b84abacf4a9e2d1f2a646a34fd2b526bab00a7073f4fd8409d28bbc7e817a32583b1649530f732366f8e41323e2ae170f5211447605ecfea72461c7d0\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"79eec14991f8f5ba11672831ad401a87764fe0ded6696011e98d4da446a6f5cc\",\"data\":\"03ee40ffd61673753ee3ce619981c9868f78e928426f4a7f9d6dc8fff32c832419\",\"tags\":[[\"locktime\",\"1781734549\"],[\"refund\",\"03f02a2a968ced0c3f00b6951e10a2c135b3a2b125e1907a4fa3bdb3bc7ce2f61f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c8a13ebd0ed8c82a5382859143a5e3cc1161497fe07df4998cfdbcd7c1d88f32"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0216bd853d7891d815051b64850118da265e5e555eed3f573a88f3967b6ed323b5"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0242f71bf738e64a652418e1b2e8a5a5739ef735ae6175dac193ad0a07ad0c8002"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_locktime_before_expiry_primary_only label=SIG_ALL_primary_before_locktime\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_locktime_after_expiry_primary_still_works(t *testing.T) {
	setCompatUnixTime(t, 1781727350)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"5d11cfc203664bbb5f23a5fb1e7c7ff0386237e9f6e128f0c482cfbd84d85585\",\"data\":\"0229c56568371c2952c5cd84fde6d68489994e8131a71b4b0dea8875f6096ba8ba\",\"tags\":[[\"locktime\",\"1781727349\"],[\"refund\",\"02c623a82851155646397fbfbabbf825440eea7227470dee3de3becea0347835be\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "039b8eb94e0cb7fad5cb35f9f26a86c728922f81e5828957d81a26ca1ed2f23466",
      "witness": "{\"signatures\":[\"b14c48f5b4f87ab5c8a07eaabd2c8ff539b7cb3c2adfd15583cc13183c5a7b48567e0704e7e6c135f3e00a1cacb03753dca1e4a96342c6c3cd2dc5df3e761165\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"819d7e2fdd5fedbd68384b36061370a70a2f515a8bcb282b0107847efab7ecc5\",\"data\":\"0229c56568371c2952c5cd84fde6d68489994e8131a71b4b0dea8875f6096ba8ba\",\"tags\":[[\"locktime\",\"1781727349\"],[\"refund\",\"02c623a82851155646397fbfbabbf825440eea7227470dee3de3becea0347835be\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02bd74904537b02d2c8d11b2312fae85e03fed3d65ac27c24ec6ec81f36422f7b9"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "023ef6cdb9ad3e9f3cc62ffbb25d84b743f63ab9180b1f524e98d20bc65711039c"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0339461cced31998e92b6829688ae801b56092551d59556e8836049184682d5d26"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_locktime_after_expiry_primary_still_works label=SIG_ALL_primary_after_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_locktime_after_expiry_no_refund_anyone_can_spend(t *testing.T) {
	setCompatUnixTime(t, 1781727350)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"634bf4e1f0852f4503d2cb805735b4df9fe4b749499a4e8220af6bf4d11dd637\",\"data\":\"0289f1e8c147f19b8ba7ca86eb9d161ed9b010a8bf41fe4173f31cdc5f05f2639f\",\"tags\":[[\"locktime\",\"1781727349\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0391a0a87facf432b00ae1586f24fda4f4602ce485857f53f0f47fb0896660ddbf"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0271fb80f85bddcf20092cc9af3374ee4e56be4a4476dbde05d2eb39c6215c9c\",\"data\":\"0289f1e8c147f19b8ba7ca86eb9d161ed9b010a8bf41fe4173f31cdc5f05f2639f\",\"tags\":[[\"locktime\",\"1781727349\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02457468ed742487053feb9edc67261ce836196be22e979c14237fcc468b0d6137"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "036565ff8105f6195b72272861d7dc9cc7324f543cc296b53e578e4e59d7a97ce7"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02dc8a0fc020d11df38fd4f62dcb281725b5cdf12d750246d24ed816bcc8e9c120"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_locktime_after_expiry_no_refund_anyone_can_spend label=SIG_ALL_anyone_can_spend\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_multisig_locktime_primary_still_works(t *testing.T) {
	setCompatUnixTime(t, 1781730851)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"2ed0b188fa18b97d905fd324865aaf8b3f959cc58f03aca8c2a85d4753d84eb3\",\"data\":\"03884b6a5a12daf991827cd24eb96c954b2eddbeb951855348cfd2ad0aa59df340\",\"tags\":[[\"pubkeys\",\"024252fdfe22388fe34ab371f3fd38f0cb7d8ff54c156a7ad616778fe87ac7c822\",\"020b1174ad6dbc315346eff562270f4dc25d567f47a39301ac83ac4a7edbbcddfa\"],[\"locktime\",\"1781730850\"],[\"n_sigs\",\"2\"],[\"refund\",\"020cfcc9f1618b40f1a0e0a439e6642341f30075752d4dc852bbcda72e70fb1d69\",\"02d108473d9230cd31b0cc038a9c46f51f6472e6c14303a8b0df4b033b3eca3d4e\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "037a004658471759019b365402c90491880988ecbcf86ab4f9c8dc83ae98ff0fa8",
      "witness": "{\"signatures\":[\"78c235368d65b8f59c9e95fec736a4f08ad690a8eb35874d41340eb2fb2789877c8f02eac99b03cb2505651d0621ca908dcd095276b59927872023ae9ac0e17e\",\"0f84ccd2df7b5725ee3e841173053cdf32d610cabea88b20d67985e28de04fd95d60a9af47793c202372e91794e8b0934fe3b8f30663f9e56ac61efb58e7dd6f\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"bd5aa9a6e2eb8108313b3a535a58b3a2dee60f552d61d2ddb6cf8df3add4014f\",\"data\":\"03884b6a5a12daf991827cd24eb96c954b2eddbeb951855348cfd2ad0aa59df340\",\"tags\":[[\"pubkeys\",\"024252fdfe22388fe34ab371f3fd38f0cb7d8ff54c156a7ad616778fe87ac7c822\",\"020b1174ad6dbc315346eff562270f4dc25d567f47a39301ac83ac4a7edbbcddfa\"],[\"locktime\",\"1781730850\"],[\"n_sigs\",\"2\"],[\"refund\",\"020cfcc9f1618b40f1a0e0a439e6642341f30075752d4dc852bbcda72e70fb1d69\",\"02d108473d9230cd31b0cc038a9c46f51f6472e6c14303a8b0df4b033b3eca3d4e\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "037264437b740b689dcc4ab11be7ecac997091597d6438a12679dca5ed9e1aa2d5"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "034a3614c8fc506fca88977cd4bb8cc16431e84e33cdbe8814dea4028dec993422"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "020e05b19b8940cf66ce1eb135458ad0e537352feaa28839b6a3a332e14adc7bde"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_locktime_primary_still_works label=SIG_ALL_primary_multisig_after_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_mixed_proofs_different_data_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"06a9de58f7fcd8ee8546d67a54463549157eb424557d603d6bd3cfca95f381d8\",\"data\":\"03ec04a2e43eb1b04fa7ffef934127848c6902b7e0cedfaa4f8d14106b51f3b6ce\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03d6db559400df5f339e31cba4020dd3de7f19443e5b33f5ced37002d6d44ed608",
      "witness": "{\"signatures\":[\"4863e32148cea88e33209b89c5417b641d21485556b59e055038f56424f5cb59903904310ea3f10380dc60294e52698301095fab3e65ebacf6506860f3173a33\",\"d0a164947fcfe6235bad78b34eb81a7f759fa33b2fd85e092e69963b728902637756ce926e20ee8072e955baea84955051dc60731e947cd760fa8c5d48dd109c\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"dab10bd128821263917eff6cd63707b3fba6fa67d328c2dc0c5ac79f342051fd\",\"data\":\"03ec04a2e43eb1b04fa7ffef934127848c6902b7e0cedfaa4f8d14106b51f3b6ce\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0245b0ab8f0a08bff16567848e071ac2830b27776f2e7c9634c7b6a74999c77daf"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"9135358de863443a8068a72fdda2fd197b362e756f4f490493d84e534e4af682\",\"data\":\"03c9971859b5232653872206d74cd4d54dd28b27a6405fb5e0a550e5a41fc6f4ab\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "029c59673bad837ee7edb97e19775b07e5d03c650c4ed2b09a4307becb1a5c44fd"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"ee8f433412ac48137e357586a0c0938a95a9dfc8029f80e9fee6d4dd567ef68a\",\"data\":\"03c9971859b5232653872206d74cd4d54dd28b27a6405fb5e0a550e5a41fc6f4ab\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0233d72e2fc7395c0889fe90b940de2a14bc42676e34944365e43abff9553c3336"
    }
  ],
  "outputs": [
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "037834b6b659fc4e90351f690f20f51c0ffbc9810962ad832b8418afda9fb7df59"
    },
    {
      "amount": 16,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02adbeb26e470f53b4e2bb6e98a99b4f6f005998212ab46e0dfb4e64e17171f23a"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_data_fail label=mixed_SIG_ALL_data\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrInvalidSpendCondition)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"06a9de58f7fcd8ee8546d67a54463549157eb424557d603d6bd3cfca95f381d8\",\"data\":\"03ec04a2e43eb1b04fa7ffef934127848c6902b7e0cedfaa4f8d14106b51f3b6ce\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03d6db559400df5f339e31cba4020dd3de7f19443e5b33f5ced37002d6d44ed608",
      "witness": "{\"signatures\":[\"8ecc79c17d3dcc67d5513b1f83e9b0a89f78630620cde1907990e6ab237644dccabbb11ee75ad7e96db3330ffbcec022c19e715f56d35edbc62d91b71e2c085f\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"dab10bd128821263917eff6cd63707b3fba6fa67d328c2dc0c5ac79f342051fd\",\"data\":\"03ec04a2e43eb1b04fa7ffef934127848c6902b7e0cedfaa4f8d14106b51f3b6ce\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0245b0ab8f0a08bff16567848e071ac2830b27776f2e7c9634c7b6a74999c77daf"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0328a16feb6c5dcaa14ef50a27431d04c1ffa091cb3096ec6494e51c2b0519b82f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02ad16191490cedf10897b8e0f8e18e62c9048d09b7438ae13f97cdaffd2f60e27"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_data_fail label=alice_only_SIG_ALL\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)

	payload3 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"9135358de863443a8068a72fdda2fd197b362e756f4f490493d84e534e4af682\",\"data\":\"03c9971859b5232653872206d74cd4d54dd28b27a6405fb5e0a550e5a41fc6f4ab\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "029c59673bad837ee7edb97e19775b07e5d03c650c4ed2b09a4307becb1a5c44fd",
      "witness": "{\"signatures\":[\"d9708e8bad2f1ec93ae8ae98b55c98a86105b5d16de42345eee89536e978a3707b9f26ea2a0932fdc329b64cb5daacfc290beafc43a10918e30d44b75f81e9b3\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"ee8f433412ac48137e357586a0c0938a95a9dfc8029f80e9fee6d4dd567ef68a\",\"data\":\"03c9971859b5232653872206d74cd4d54dd28b27a6405fb5e0a550e5a41fc6f4ab\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0233d72e2fc7395c0889fe90b940de2a14bc42676e34944365e43abff9553c3336"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03cfdbab2906509c75b5a2a0ac1f263423d9c5030434cf5662faa49a215c3a3c78"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d9036ac2fd7f8f1efd23c2d33755070ab1411c4c452bed6d5317b2b7162cc7f7"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_data_fail label=bob_only_SIG_ALL\n%s", payload3)
	request3 := decodeSwapRequest(t, payload3)
	err = validateSwapRequestForCompat(request3)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_mixed_proofs_different_kind_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"560380512b3dd54ae46db4da620d0fa8649beeb3825dbae566a545e8df34c587\",\"data\":\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0231c19bceb658216c8005ccee8e9186753298274776142df534c62bdd16bd1af4"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"e6e308471380ab470b90ac2daaa268a8e780015aa785fa469735c34128414215\",\"data\":\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "023bf845ad3d41a228892d2a568eb10e61aa4b2a0e274b4cf817c714ab9cea943f"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"56ebb5a28fb3bfdadf02e4c18e74fff0fa199dac7ca254a7f47a8a32bdd08741\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "039eedb4a37432e258bbb8f7ede0bb373bba37b083a4b8a003209d551047e7d018"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b91bd907656a415fee61d94906a6fe038c892f2e0b99423ffa6765864bf041c2\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03f6a315c692f958ef056ec484923a49a21f82f627af8a37f71a2be967db6ac33c"
    }
  ],
  "outputs": [
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d2712c843fde26810e58872b74b60d4894d37e58f65d0052ee7c23b9700b9bdb"
    },
    {
      "amount": 16,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03221cc475222d69285b47d4dd98ec578b0d9d0a7e11a28ef325a5c3362de035d6"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_kind_fail label=mixed_SIG_ALL_kind\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrInvalidSpendCondition)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"560380512b3dd54ae46db4da620d0fa8649beeb3825dbae566a545e8df34c587\",\"data\":\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0231c19bceb658216c8005ccee8e9186753298274776142df534c62bdd16bd1af4",
      "witness": "{\"signatures\":[\"c5e4a1ec60d81b49e461337faedade95912e2c671ae11804f0d3b9983897f32d7a1d7b3d69ea676226bfa65c01f688367d4436c1327bcc093d9cf97106ce81db\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"e6e308471380ab470b90ac2daaa268a8e780015aa785fa469735c34128414215\",\"data\":\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "023bf845ad3d41a228892d2a568eb10e61aa4b2a0e274b4cf817c714ab9cea943f"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "036893a596f36f3a6b8e36aeddbf31c6dfed1de2846910817464a73d227d9fb21d"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0387a0bc0853d4b3e36802e18d95c7c16f8ab73d9eec622e4e4edc0df6b73ec641"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_kind_fail label=p2pk_only_mixed_kind_control\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)

	payload3 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"56ebb5a28fb3bfdadf02e4c18e74fff0fa199dac7ca254a7f47a8a32bdd08741\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "039eedb4a37432e258bbb8f7ede0bb373bba37b083a4b8a003209d551047e7d018",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"1da9b115b99d33df5a7effddf459f7da02eab4cb5f1b71eeed8a1c02c0ca47c257ae4612f30148e6430774faa1199683fa730b379e1b6f8020d1f030c02a3eda\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b91bd907656a415fee61d94906a6fe038c892f2e0b99423ffa6765864bf041c2\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"027b59d551f5eb1e00ef4a7514887580b1f56bde6c855c39b00bc4e5ce9ea08bbf\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03f6a315c692f958ef056ec484923a49a21f82f627af8a37f71a2be967db6ac33c"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "035c0a7256bf3757cf7708cf6b1ad36695c05bc992a5523b18b0ddb47cdcafe405"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c096dee654be0e9dd78be256515292448e2e81c1da659411eee6709e721aa87d"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_kind_fail label=htlc_only_mixed_kind_control\n%s", payload3)
	request3 := decodeSwapRequest(t, payload3)
	err = validateSwapRequestForCompat(request3)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_mixed_proofs_different_tags_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"2b492c8f60b53f52f6e4ed1a5baec7611d715774d1fb884d6564e6c4215f5323\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0324d903f5545cd0287d410a0106ff755feeb35a484aaa01fe49b37f3987cc3497"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"6d84789b09e0e4c55e08f44e76622e07ddb5e2e30cc03109ac38451db4cf47ec\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "025434b4e951bd055225ea9b84480a160ba4ba645bd2dd639195dca698ce8bd487"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f1dbb8647c56939bea627277d610cdc6305b0b0c3f75d233f52fd7c2bad6650b\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"locktime\",\"1781734551\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02ee0e12f91ce175d3e1aa99f013f979e26a8fe6268aff147c3c95868c86e00ac4"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"bc0d00d6aebe5448e3ae9539e788690992ffa2ad7632ac52afc4e3f431c55a01\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"locktime\",\"1781734551\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03e63a97dc660aca4f2c3e4fff9d8638631579971297b26a171dbdd2777c74e5ce"
    }
  ],
  "outputs": [
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "025ed67222fbfb4eb546031d7dec507d62452be24adff1cf4c6bd928a79fffc95a"
    },
    {
      "amount": 16,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "032c84d13452b6613d1f01ea41168a1e1eb909a03bbf795fe23c2c8c6bb854af17"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_tags_fail label=mixed_SIG_ALL_tags\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrInvalidSpendCondition)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"2b492c8f60b53f52f6e4ed1a5baec7611d715774d1fb884d6564e6c4215f5323\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0324d903f5545cd0287d410a0106ff755feeb35a484aaa01fe49b37f3987cc3497",
      "witness": "{\"signatures\":[\"aa9a645f491b48d815e909ea81e0a4b964c9704d87fe3e893a09ffcae1e80742dd5187afbe5d7c312744ddd427cfaf3fe55b558e23686f716f4ce007a45185ed\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"6d84789b09e0e4c55e08f44e76622e07ddb5e2e30cc03109ac38451db4cf47ec\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "025434b4e951bd055225ea9b84480a160ba4ba645bd2dd639195dca698ce8bd487"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0296453f65e431f252630dfd9f76f429e49b48867826b7be2204d6b1b2db11608e"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02070ec51d9bfa1115952af0673dc88da443109d4c4fc58cdd28d0b642ad6f792d"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_tags_fail label=plain_only_mixed_tags_control\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)

	payload3 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f1dbb8647c56939bea627277d610cdc6305b0b0c3f75d233f52fd7c2bad6650b\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"locktime\",\"1781734551\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02ee0e12f91ce175d3e1aa99f013f979e26a8fe6268aff147c3c95868c86e00ac4",
      "witness": "{\"signatures\":[\"0e3ef93163fb119ba5dfbefaf488eec4d402e39e87f9e2a9eef7c6c381244ba927ba8ade4ee1c7b395f50f469c410fcdf3a5ef1bcfae2ebd0b72c3e3c2651b01\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"bc0d00d6aebe5448e3ae9539e788690992ffa2ad7632ac52afc4e3f431c55a01\",\"data\":\"0321f46ec408b929ada27a5b340c6e77dff8b5394b437d3ea586081d06a923c596\",\"tags\":[[\"locktime\",\"1781734551\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03e63a97dc660aca4f2c3e4fff9d8638631579971297b26a171dbdd2777c74e5ce"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02db8d54364a0a30688df7d453ff75edcb9f127b5a10cb9406c2c490bbea3f1cef"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03ca686bb13bb2498b927845063c82100cdfc20fac8fe4ca167684c61dc011aa8f"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_mixed_proofs_different_tags_fail label=tagged_only_mixed_tags_control\n%s", payload3)
	request3 := decodeSwapRequest(t, payload3)
	err = validateSwapRequestForCompat(request3)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_multisig_before_locktime(t *testing.T) {
	setCompatUnixTime(t, 1781734551)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"a4d46f31474c771c24b8960ff0be7375ec99fc3f8d3c3cb41471806e6a21bdea\",\"data\":\"034c7a04055d325baef458d9e96c8a841f054163479588d8ca02dfb3c4f55b3280\",\"tags\":[[\"pubkeys\",\"028021ea7e326460060fbfab59b2133a845e0349890026820685b9f24119b473fb\",\"021c146a591f6f1dc28566a359f12f6f44d65add1504199eb3f815cdd853643f58\"],[\"locktime\",\"1781734552\"],[\"n_sigs\",\"2\"],[\"refund\",\"03b4a173b7bcc7222bd46497a6904a011bd32b970f13965ae94d4324f78f1c40ad\",\"02b7b776067ea33af217848518e5c0f2feb277f25c3f852439e6be031dfa9d9883\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "028e94729c49fd7e869ce97945886bf1a9da3bfe825bed4e3496bb9d88c1c78663",
      "witness": "{\"signatures\":[\"0e494530e9b93bad4edba47d282d3ad63ec551e5bb021701b0002571e84775e2ddf6ac99c9b33d1ef658405205b600e4d92e88c44bfb5518090d3358344e6c3a\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"19ae7e7fbf9e4d8081ed6cfa02b0b679775504298d0a0530057eb7edc3d70b3b\",\"data\":\"034c7a04055d325baef458d9e96c8a841f054163479588d8ca02dfb3c4f55b3280\",\"tags\":[[\"pubkeys\",\"028021ea7e326460060fbfab59b2133a845e0349890026820685b9f24119b473fb\",\"021c146a591f6f1dc28566a359f12f6f44d65add1504199eb3f815cdd853643f58\"],[\"locktime\",\"1781734552\"],[\"n_sigs\",\"2\"],[\"refund\",\"03b4a173b7bcc7222bd46497a6904a011bd32b970f13965ae94d4324f78f1c40ad\",\"02b7b776067ea33af217848518e5c0f2feb277f25c3f852439e6be031dfa9d9883\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "020eca7338e8e868e954608bba623cbd240fdc0de6640d7fe039ea3f6f777f8cf0"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03bdacec658c9c7a1ec7a69dfae21cce0952d8e887613735a8f76f3dcc5dd804eb"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "038acd84fd99773a9f3cf4bd5429e08fb4444b9f1971ed91af642b7056572c5a06"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_before_locktime label=SIG_ALL_1_of_3_before_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"a4d46f31474c771c24b8960ff0be7375ec99fc3f8d3c3cb41471806e6a21bdea\",\"data\":\"034c7a04055d325baef458d9e96c8a841f054163479588d8ca02dfb3c4f55b3280\",\"tags\":[[\"pubkeys\",\"028021ea7e326460060fbfab59b2133a845e0349890026820685b9f24119b473fb\",\"021c146a591f6f1dc28566a359f12f6f44d65add1504199eb3f815cdd853643f58\"],[\"locktime\",\"1781734552\"],[\"n_sigs\",\"2\"],[\"refund\",\"03b4a173b7bcc7222bd46497a6904a011bd32b970f13965ae94d4324f78f1c40ad\",\"02b7b776067ea33af217848518e5c0f2feb277f25c3f852439e6be031dfa9d9883\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "028e94729c49fd7e869ce97945886bf1a9da3bfe825bed4e3496bb9d88c1c78663",
      "witness": "{\"signatures\":[\"fbc219a5e176d3b968110293b5e4e1366badb1c4303b466627f12abdc036b94172025075ff61e012c2b4e3c982c0eee44a2e2640b492a7e2801d760a4f6c5c81\",\"f3770f2ba2a232882cc132cd3bdb45b237ca5ff3b724801915b7642a13fdc77f6e01ab3c82c3a5336969990e19e781bea2697a0cd8bbd6c79de3587cbc843f35\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"19ae7e7fbf9e4d8081ed6cfa02b0b679775504298d0a0530057eb7edc3d70b3b\",\"data\":\"034c7a04055d325baef458d9e96c8a841f054163479588d8ca02dfb3c4f55b3280\",\"tags\":[[\"pubkeys\",\"028021ea7e326460060fbfab59b2133a845e0349890026820685b9f24119b473fb\",\"021c146a591f6f1dc28566a359f12f6f44d65add1504199eb3f815cdd853643f58\"],[\"locktime\",\"1781734552\"],[\"n_sigs\",\"2\"],[\"refund\",\"03b4a173b7bcc7222bd46497a6904a011bd32b970f13965ae94d4324f78f1c40ad\",\"02b7b776067ea33af217848518e5c0f2feb277f25c3f852439e6be031dfa9d9883\"],[\"n_sigs_refund\",\"1\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "020eca7338e8e868e954608bba623cbd240fdc0de6640d7fe039ea3f6f777f8cf0"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03bdacec658c9c7a1ec7a69dfae21cce0952d8e887613735a8f76f3dcc5dd804eb"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "038acd84fd99773a9f3cf4bd5429e08fb4444b9f1971ed91af642b7056572c5a06"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_multisig_before_locktime label=SIG_ALL_2_of_3_before_locktime\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_more_signatures_than_required(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"b546e432689ae743ca621d12b249e73a92906f5ef088873f234264bd7a3270df\",\"data\":\"036c270ddd755c185b9be4768ee3d56fb7ed6d6d084e8d0ec0480b947e6b4ffcec\",\"tags\":[[\"pubkeys\",\"02b81f8f01ffe04467ffa6b109c0da9132dcff645f7c19110027e757e14f54efea\",\"03d72d3de6fad0e4b6e6bb39dcb8a8a2fb3a7fd991a6f88ceb6c0d5324c1cab86c\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03ccf8148c6c7f5a36c01f20492860af41e1159ce9032631c66f067ada77552365",
      "witness": "{\"signatures\":[\"800f63d6d5a99e762c3cf1313e1669f94691348698ca3fde172227013a0b72a2fdf6f26cb65419a2758ada878fb303f34d2fb47e22f57714a22a244a02fbd0f9\",\"ca625f8721d0b7294f0017a23fcbae76e03c0b6f8abad7b7701de0aea9aead8add3e455da5c8916ff3e25f0365268a42f3293356c929fca8813daabefe365647\",\"6f5f9ce744f6e4ec9dcfb017b0b9d80201ac3cf3cb381926bbdaa22dca0cf9c4b59f49e8bc79b06f88c3b399782ae3c425501babc5a7a61511864733022b6ee7\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"4c1aaceaa62b86d2522f2ad11163347b84c906191067c81102f97ac37e1d3c9f\",\"data\":\"036c270ddd755c185b9be4768ee3d56fb7ed6d6d084e8d0ec0480b947e6b4ffcec\",\"tags\":[[\"pubkeys\",\"02b81f8f01ffe04467ffa6b109c0da9132dcff645f7c19110027e757e14f54efea\",\"03d72d3de6fad0e4b6e6bb39dcb8a8a2fb3a7fd991a6f88ceb6c0d5324c1cab86c\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02a283c0ee98e0321d010170c3b7bc74a53130221fb7fb14c393ce09c50af8f73c"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0238858540b651ef75cd8ee66d8342141e08115e841b28204736f9c903f0f8523a"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "039a51911724a1cc0693494d3cd3e099592713fbc9f4e333f4726de83bee62c4b9"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_more_signatures_than_required label=extra_valid_signatures\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_refund_multisig_2of2(t *testing.T) {
	setCompatUnixTime(t, 1781727353)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f7c7c1cec426171f253ebee2d8e417d2d2e379073b969188b0d8856d796711d9\",\"data\":\"02a3f61cf8d4f6d77c16f689d802a5131a843342f7989d0a19ebd81fbae15f6264\",\"tags\":[[\"locktime\",\"1781727352\"],[\"refund\",\"03d489b0cc07506be87ecd4389fd92ae1fbe004421082ebffee34bc3a6ed320c84\",\"023a67d6b3c0dcd36dee89dcf3965568018a876690798d49b2a22a3c1fa0b9b448\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02242443a158798bd237ce2f384ffb8cffbcfe9ae54399dbe2c1c5229cae4d7b48",
      "witness": "{\"signatures\":[\"dbca92d82655f40a232163e01dda742f04adbca9894b41bbfcfbd3d99d07403e7a65304ed9299d13ca9d8c54d22d555debaf8c619811433d59969253d468be7e\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f4a78702b4e811cb23561a82160f855c81e477a7a3e4eaaf30c4d2d96cc3d2ac\",\"data\":\"02a3f61cf8d4f6d77c16f689d802a5131a843342f7989d0a19ebd81fbae15f6264\",\"tags\":[[\"locktime\",\"1781727352\"],[\"refund\",\"03d489b0cc07506be87ecd4389fd92ae1fbe004421082ebffee34bc3a6ed320c84\",\"023a67d6b3c0dcd36dee89dcf3965568018a876690798d49b2a22a3c1fa0b9b448\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "039ebf1c761de3feffb640f3ab96691420955bc3e906983b625b87f4b0a68e56ff"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03f5f4bf2397d7304142f167f019a9d9835e4a2fe995872c8d73c7792a7d4e24af"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "028ac27168b6ea77377b2d50866e9b5337ae2b81963475388b75b4dc4f63fc6540"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_refund_multisig_2of2 label=1_of_2_refund_multisig\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f7c7c1cec426171f253ebee2d8e417d2d2e379073b969188b0d8856d796711d9\",\"data\":\"02a3f61cf8d4f6d77c16f689d802a5131a843342f7989d0a19ebd81fbae15f6264\",\"tags\":[[\"locktime\",\"1781727352\"],[\"refund\",\"03d489b0cc07506be87ecd4389fd92ae1fbe004421082ebffee34bc3a6ed320c84\",\"023a67d6b3c0dcd36dee89dcf3965568018a876690798d49b2a22a3c1fa0b9b448\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02242443a158798bd237ce2f384ffb8cffbcfe9ae54399dbe2c1c5229cae4d7b48",
      "witness": "{\"signatures\":[\"34b665963d4329c16feb694f04d991dc588965c532a8647cd11ce518cfb0fd0b8249868bfc6a762adf0398db798cd639c85722a01e242dbe3722145c1e32fd69\",\"acc27d20527f04963c97959d9194b18a0a559dc6efd009c22fcdd377421ee342995ef5e40fd276a6d80f3f050c0d400ae684817b123a01eca74993b1ea743483\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"f4a78702b4e811cb23561a82160f855c81e477a7a3e4eaaf30c4d2d96cc3d2ac\",\"data\":\"02a3f61cf8d4f6d77c16f689d802a5131a843342f7989d0a19ebd81fbae15f6264\",\"tags\":[[\"locktime\",\"1781727352\"],[\"refund\",\"03d489b0cc07506be87ecd4389fd92ae1fbe004421082ebffee34bc3a6ed320c84\",\"023a67d6b3c0dcd36dee89dcf3965568018a876690798d49b2a22a3c1fa0b9b448\"],[\"n_sigs_refund\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "039ebf1c761de3feffb640f3ab96691420955bc3e906983b625b87f4b0a68e56ff"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03f5f4bf2397d7304142f167f019a9d9835e4a2fe995872c8d73c7792a7d4e24af"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "028ac27168b6ea77377b2d50866e9b5337ae2b81963475388b75b4dc4f63fc6540"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_refund_multisig_2of2 label=2_of_2_refund_multisig\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_p2pk_sigall_output_amounts_swapped_fail(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"2f07af9333c4e9068c0a84a5683bbf4706e267ccc8c03de7964fa5e2ef448c43\",\"data\":\"02eb06c6ffe0c59a49f400939ce885611e8149ad2ab8dc95c737e67b4e8aa1d664\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026a95b9470ec63a0d69210b4d4980edbc255b16604af75b875979bcc63fd8ae32",
      "witness": "{\"signatures\":[\"dd0e0abbcb0aa9b8fd2657a0df58b67b175d72e5f4dcab594ab158b33d7ac82730e40db2cf6fa597efbc20b743142cf6c4a33196059197525e5c567d1b032512\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"3684d70094ed848e25680eeddcdf83f2bbcdc5d33146d586bd5a33201a92101d\",\"data\":\"02eb06c6ffe0c59a49f400939ce885611e8149ad2ab8dc95c737e67b4e8aa1d664\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03463cdc5b6fdc8ea1fe610ff3502b6e1e1c6616cf9258f3c280e0147864131b3f"
    }
  ],
  "outputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "021ff6e7905e7dfbe8f3e530bac00fe98ff73fb541365de074bf187e06b9300f72"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d927f37be1092cf05f1eddaaef49504921c0caff14bf957f3b95083dc716405e"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_output_amounts_swapped_fail label=tampered_output_amounts\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"2f07af9333c4e9068c0a84a5683bbf4706e267ccc8c03de7964fa5e2ef448c43\",\"data\":\"02eb06c6ffe0c59a49f400939ce885611e8149ad2ab8dc95c737e67b4e8aa1d664\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026a95b9470ec63a0d69210b4d4980edbc255b16604af75b875979bcc63fd8ae32",
      "witness": "{\"signatures\":[\"dd0e0abbcb0aa9b8fd2657a0df58b67b175d72e5f4dcab594ab158b33d7ac82730e40db2cf6fa597efbc20b743142cf6c4a33196059197525e5c567d1b032512\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"3684d70094ed848e25680eeddcdf83f2bbcdc5d33146d586bd5a33201a92101d\",\"data\":\"02eb06c6ffe0c59a49f400939ce885611e8149ad2ab8dc95c737e67b4e8aa1d664\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03463cdc5b6fdc8ea1fe610ff3502b6e1e1c6616cf9258f3c280e0147864131b3f"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "021ff6e7905e7dfbe8f3e530bac00fe98ff73fb541365de074bf187e06b9300f72"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03d927f37be1092cf05f1eddaaef49504921c0caff14bf957f3b95083dc716405e"
    }
  ]
}
`)
	t.Logf("scenario=p2pk_sigall_output_amounts_swapped_fail label=restored_original_output_amounts\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_sigall_preimage_only_no_pubkeys_succeeds(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"2d46b7d8be7adee9ab4d17b4a0fa9261f1eec1f7385a1ee98bda6e120fbd9a66\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0328180ac44627a9138295d38f482c0925385f021c1c35f5106a487eeabf213348",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b110d020abc498b1d324bfdcb591f35257a0bde59c3b0a63583d8dee0096ff63\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03728d412c63363b77686dc32dc309b95a05328b2005a7fce324955b1e07bdc152"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03ddbc5b39bb7a563dc2d51b23eed04f66d38bca74664ed899c39e722190b6d0fe"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03babf0f81797a94605746900e5b2e33e1b8914c99a8b271e782119272c69b444f"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_preimage_only_no_pubkeys_succeeds label=SIG_ALL_HTLC_preimage_only_without_pubkeys\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_sigall_preimage_only_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"bc045e509f52016662f452114ed823f77e0da4618302649bc48e650b3cc5a72e\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02a39d309f35bb761eeac69b63eda996b2cac8118c07978b382bba5b450a122952\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03d0f632bb7f2af21de427cbddc6b87b5d49c1b4d510d13cf2945985ec1fb09f32",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"40617a7c62950875d06128bcb9982939588294d7d758722000113efc0e2bde20\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02a39d309f35bb761eeac69b63eda996b2cac8118c07978b382bba5b450a122952\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0317bda395bdd0208f72b9825ff4585db97d6d9f67fb776b968af5501a234cd761"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "034af10ab9b2426b965962e102c5813d63c147190b86edf1b319226e65db4f6c58"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02a4138fd1f8da639daad8ae5894deac2fffc42d0c11165f92d9d57db662690f85"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_preimage_only_fails label=SIG_ALL_HTLC_preimage_only\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_htlc_sigall_signature_only_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"c4b0f1d2b34393227cc597ef31e8a176f536c75434460a18d5443c3b79496fcf\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03a4c3d4e6216d3de58cf295d80c7a79d69d0e0b6df90d12339e252916d8e96c55\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03a4c3bf2347aec6ceb5042c88821d436d4e4af74f9e33ee980fd17d32af2d65c7",
      "witness": "{\"preimage\":\"\",\"signatures\":[\"a1815092dc6df7102a468f1dd402349377d5d8bfb22dd1fa6bacfa234d44e04070e7af5d96ffa2df3f9962e7f6201da8f0eaf134c7dd56d8d29da8f3429563be\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"341cae7c08bfebc0675983ffdfbe41aea421ca421c47862d8d0c7aefe06ad394\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03a4c3d4e6216d3de58cf295d80c7a79d69d0e0b6df90d12339e252916d8e96c55\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0288cf389a170ae16d8a4cd06308c3f53ce5d484b1889e2e9357d9a88e7a2b8462"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02c90d19bb9b4871db1e84909db5712280509a2907d95b14faffbaa5507b82f823"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0312c48e7e4ddbe5f3ab4d4deb2913cd3ddbcc5b06c5d6a401da68c55d257dd96d"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_signature_only_fails label=SIG_ALL_HTLC_signature_only\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrInvalidPreimage)
}

func TestCompatibility_htlc_sigall_requires_preimage_and_transaction_signature(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"93ea4e70d5f1324d5fffd089e86de3709e4145ed46b0731e007e043ba0e2416e\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03df39dfa9f3632b675073ba3d81701d9dfc18717619dbf8376617fcdec8a0c0fe\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c0a62e1d6bb3f98b74ce9647f1ef2a489d4da195b30ad84e1bf6c839f63400be",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"60c91d0a6145bd52d7059359558c120c312253b6cc1a756a76655e1fbe5e1b710c206d94fd4e25a07c9c2b5d5cacb5061644691a3194574c4853331d64b8de02\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"114e6ca96451d559349450b353762702827a2dc0810da6bb758586f8dd2ab1b2\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03df39dfa9f3632b675073ba3d81701d9dfc18717619dbf8376617fcdec8a0c0fe\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0378ba73bb8423398cb46036f15d6d2c3ad64ca1248945065f3add43c3a8af984e"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "032b661c025a0224f74b082c8aaa8f23c8139aba3efa5747436f7c2d13c97dcd55"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "035e65535733607f1ef0d9696ddd36bf3a9aff3e3ca1a284437ed93ff068abfa36"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_requires_preimage_and_transaction_signature label=SIG_ALL_HTLC_valid_spend\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_sigall_wrong_preimage_fails(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"bd98f3e8514a9835859c7bc0606d3f8e807c4bd8a27aa95e5181ebedf32be221\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02f9aff5582e974054033bf8b02ff929945d2dd9ffbe88fc806d3f2f837b504118\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "022668977d292d8a09469a530a30a8bc2cbcac13e2501d0a0b20e7407c9049ec5d",
      "witness": "{\"preimage\":\"this_is_the_wrong_preimage\",\"signatures\":[\"15c1d4262e0a4fbc43c3cd2a290c70d5fea09921913a7f63c9716eeea922d0fa2027d35ed17588c51c6a1018dd1820b6e2956b36a16013defcb26f46c9b4069c\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"0d4a7560c7242358e95e0083d8051aea49c50f270a7e9aaa38ab2315b86e2655\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02f9aff5582e974054033bf8b02ff929945d2dd9ffbe88fc806d3f2f837b504118\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0297d374dc55702b74d330987242bc80c21f941919965800832d21a7a2d8970e37"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "03c847febbb213f924446826df6d252fba0cb23608ffb9ef0d6d798ed7f244386e"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "029e3263bf0dbcec369842c0d344808e396b966112c09b8bf1c523e9b4de8de1a7"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_wrong_preimage_fails label=SIG_ALL_HTLC_wrong_preimage\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrInvalidHexPreimage)
}

func TestCompatibility_htlc_sigall_locktime_after_expiry_refund_succeeds(t *testing.T) {
	setCompatUnixTime(t, 1781729955)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"33482ba23eb25ca931a9cb6be48c37d3d5b0757b001059a8b9391ad6251413ee\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"036982e8fcdfe6e3b96992b5428d628192d81adb5053189305a3aef46c359163e0\"],[\"locktime\",\"1781729954\"],[\"refund\",\"0241ec6d7eefab43fd4f8a336acb336957689911a59564d9ff71be070c3f22829f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0211629bad1b3ef603af8891f7c4fbe79a2a26679199fd13c66b8cf196826d6971",
      "witness": "{\"preimage\":\"\",\"signatures\":[\"d6856931c99c2b2516f4687d3dbad5244a9e7db1c499cd97341b3610b177bfd89915eee7259a820453df0840ee6fba30b43074805d9d766d100105bfbe70a2b6\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"b3286541a6f747e19a6b8ca9db349beebf4d0809297c08391af71ee712f04954\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"036982e8fcdfe6e3b96992b5428d628192d81adb5053189305a3aef46c359163e0\"],[\"locktime\",\"1781729954\"],[\"refund\",\"0241ec6d7eefab43fd4f8a336acb336957689911a59564d9ff71be070c3f22829f\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02ee15560b33ddb258aada971a2b52d7c243d04c5a9623168598c2eba259222b60"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0336248e0f8c3bdcf9ebccd6c6ab527dffb43f21a07ae5eb1d520d45e2c1993b82"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02eaa5de5287a84838b5b8fd2b9a5721f6be1ddf38b83a0c8eb601f2ce0cee80cb"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_locktime_after_expiry_refund_succeeds label=SIG_ALL_HTLC_refund_after_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_sigall_multisig_2of3(t *testing.T) {
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"56e61c12b585df27630c6871739cf6438158ef0df94accb4194158c55713a66f\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03152b40a455a50e732741b25ada824fea022c9e352edd943969e41b48ab92226e\",\"029f3140be040ea9b1247585722b9707f74598f1e818d0dfc1217e1047ed48db5e\",\"0204fb683bd3202f5cc1f7edb0b439cb86ee3bf3c500b92336177f292e7d651c7b\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "021d1df1c390e9b79817676fe2e912e842fe6370988cd05dd770f5aa31b6863b7b",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"ede4741a9237388811fecfbc86ed4ad932191d80a17993c834142daf32d34a7a0e181186d029269f13e03783f43616aea2e9ea48adda0d761eab495d99e7d8a5\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"db1214ccd53a1c58bbf92a0f8008fd7f493f82820882429ae9337533d698b768\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03152b40a455a50e732741b25ada824fea022c9e352edd943969e41b48ab92226e\",\"029f3140be040ea9b1247585722b9707f74598f1e818d0dfc1217e1047ed48db5e\",\"0204fb683bd3202f5cc1f7edb0b439cb86ee3bf3c500b92336177f292e7d651c7b\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03e9046db8c959f98720b128a353e248cc4927e4d81d4e83064d838bf8e635b9ab"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02586dcd622936fc6af6628731bf1c24d05d3e7172cc0b56566350030047cf10c9"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02fcf5ec62685c6b97a3c7d00fdc4649779eeba1289547a43838ae1be62275d96d"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_multisig_2of3 label=SIG_ALL_HTLC_1_of_3\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)

	payload2 := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"56e61c12b585df27630c6871739cf6438158ef0df94accb4194158c55713a66f\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03152b40a455a50e732741b25ada824fea022c9e352edd943969e41b48ab92226e\",\"029f3140be040ea9b1247585722b9707f74598f1e818d0dfc1217e1047ed48db5e\",\"0204fb683bd3202f5cc1f7edb0b439cb86ee3bf3c500b92336177f292e7d651c7b\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "021d1df1c390e9b79817676fe2e912e842fe6370988cd05dd770f5aa31b6863b7b",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"5b289bf55a6e264bafd7b257bfcd35920b388a6920816bf954e8062d6c6fb445901982e63ee4377c11193da6185b3ac609dc1f940e512dbd2776a4aa3b109b60\",\"85b9ea0cec695414412289b37ffe4a2348beccc82c5f1ec6eed8279bd1c146a1bc178191edfcefe0efbbdcfef59f7db6d4ad6a5cbcd140ac5cb70d53543c8697\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"db1214ccd53a1c58bbf92a0f8008fd7f493f82820882429ae9337533d698b768\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03152b40a455a50e732741b25ada824fea022c9e352edd943969e41b48ab92226e\",\"029f3140be040ea9b1247585722b9707f74598f1e818d0dfc1217e1047ed48db5e\",\"0204fb683bd3202f5cc1f7edb0b439cb86ee3bf3c500b92336177f292e7d651c7b\"],[\"n_sigs\",\"2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03e9046db8c959f98720b128a353e248cc4927e4d81d4e83064d838bf8e635b9ab"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02586dcd622936fc6af6628731bf1c24d05d3e7172cc0b56566350030047cf10c9"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "02fcf5ec62685c6b97a3c7d00fdc4649779eeba1289547a43838ae1be62275d96d"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_multisig_2of3 label=SIG_ALL_HTLC_2_of_3\n%s", payload2)
	request2 := decodeSwapRequest(t, payload2)
	err = validateSwapRequestForCompat(request2)
	assertCompatError(t, err, nil)
}

func TestCompatibility_htlc_sigall_receiver_path_after_locktime(t *testing.T) {
	setCompatUnixTime(t, 1781729955)
	payload := []byte(`{
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"19792005ce11bcca9e13695eb3a4a5f4494c67eb0673d20ac1fdffa0387bff5b\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0268ba663639f7f9cda9d7c4540ec3e05716b34d9b81a092599bd16a8ae109cfc3\"],[\"locktime\",\"1781729954\"],[\"refund\",\"034130d16b7e150956ea691b5a753e9663e8b312682f9ee17543ff796ff62f18d5\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02a38c7ea9d9be42ec484a79dcd90e9e742afa95cbfc662cd461cf77e4c283c56d",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"c07692c985895cd6a6c3691c0a4e9ec1bc646d767017c0a9ab51cd9ca75e29e00e03b46c1a3de901b13a88effddef29b77c4315ee2587bcb7941c3788c7cdff3\"]}"
    },
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"93637545cad534076c36dd487874d15b2ad62ddfad71811c63318bb2af93b4a1\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"0268ba663639f7f9cda9d7c4540ec3e05716b34d9b81a092599bd16a8ae109cfc3\"],[\"locktime\",\"1781729954\"],[\"refund\",\"034130d16b7e150956ea691b5a753e9663e8b312682f9ee17543ff796ff62f18d5\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "026b45de7452a17409f104f73d27b7341ae73573b8214754289d3050fc87bc5eb0"
    }
  ],
  "outputs": [
    {
      "amount": 2,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0262119d63f0ceeaaa0a92bdba5d95bc61fd033908615fc1fa311cfabc22711394"
    },
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "B_": "0203312f081329583f39709c0f7eb84f9b7aa8a9393c19dfda9338fcd872d28c32"
    }
  ]
}
`)
	t.Logf("scenario=htlc_sigall_receiver_path_after_locktime label=SIG_ALL_HTLC_receiver_after_locktime\n%s", payload)
	request1 := decodeSwapRequest(t, payload)
	err := validateSwapRequestForCompat(request1)
	assertCompatError(t, err, nil)
}
