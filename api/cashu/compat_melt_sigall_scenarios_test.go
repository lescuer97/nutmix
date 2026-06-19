package cashu

import "testing"

func TestCompatibility_melt_p2pk_sigall_unsigned_fails(t *testing.T) {
	payload := []byte(`{
  "quote": "ULhtduLerz8ekrdAeo-XEfIli_CZ1M5CnEKg5Wwf",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"ceb30c1d9a04e91bc7e0aab532791a0cccb9c97079c3641c36d8b6c78f319c33\",\"data\":\"02ab65f50ab5c4cc106041d686ab91b82c7bfcc6d7bb6dfa529f2694d514d752cb\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02690ea806c18255c047f8f82ea3860f9091444ffe5320912b3c0e3d2cef160503"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"0d59ff41253755a71113757445f49344b6b503df8a674c829bc9be7c7e36010a\",\"data\":\"02ab65f50ab5c4cc106041d686ab91b82c7bfcc6d7bb6dfa529f2694d514d752cb\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0360449757382b80d4e0a869fa429f95a129a8d1746da333fc58fbd260972ca1d0"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_sigall_unsigned_fails label=melt_P2PK_SIG_ALL_unsigned\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_melt_p2pk_sigall_sig_inputs_fail(t *testing.T) {
	payload := []byte(`{
  "quote": "c_2sg6Ipsqkt4baNszjh59XQwOtrym_gTnHk6tNE",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"161dd7adcb4b7a18c6f395bf849d8b201cd6cab9494a32917536d9ac2f8f993f\",\"data\":\"03c4a8af6aaf0f49af5556a047c9ec38483ee08a7de5bbcad9f1105906c86230df\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "031ceba7ddeb8c544811f0153b1b3c013b5badcbc367169119eedb8fc2cd86b3be",
      "witness": "{\"signatures\":[\"b04a25301cdab6161affed1501b508ca8bb37d15ed03a3e2602d8e681506bc7a2f9db1d80db7bd9076a15253cd4fc4ed6b19531b96d79613466dae908804a0b6\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"a73d6231e6e3647c4b6d6abd52fa72e5f755fc4ba9e0315402220277c2c1fa8a\",\"data\":\"03c4a8af6aaf0f49af5556a047c9ec38483ee08a7de5bbcad9f1105906c86230df\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02b317ebc9de5c3105dad7c7448c54325cdab30d846496dc2220cae39a01dbf093",
      "witness": "{\"signatures\":[\"9ef488ba1e62299896e6e180a752fccc35f41a37dd611d0c761f6dd577e0c7ba3f42f92789071e6453c4181a269cb21ed1691eec089653912b83e94d2da95be2\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_sigall_sig_inputs_fail label=melt_P2PK_SIG_ALL_sig_inputs\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_melt_p2pk_sigall_transaction_signature_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "ZYyOhmwSPctq5zmUk_DiNQ_BZnpTPH9hMenBovPN",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"802a03ad1daad1fadd4e2f54030797629b4524cfd09ea785869c6df42948535c\",\"data\":\"033b987d4730010dc06a7215527195b6fe0ee977c06bfc1d397f2ac5be9a964043\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03b917317dcd5c0fa18760b5f82f60210c28446c15b3ee9753daedcf2a85dd132a",
      "witness": "{\"signatures\":[\"c6e2581c3fdceb20eeb0643152c7033ce98eaf3078ecbea4b3b0e892ac870dfaec0476dd0919e942c7f02ba637bd58cb1a5d35398bed135cc6a5e472d4cb09b4\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"P2PK\",{\"nonce\":\"c3ab0956f6dc5ed639bb842f6bd755307e7b69cf0f29922c4c5c1c023cdf2b09\",\"data\":\"033b987d4730010dc06a7215527195b6fe0ee977c06bfc1d397f2ac5be9a964043\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03981b658927fbbc3e52b9f796b2ca137a7f70d2cd3a5b11b51fefdedf1463844b"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_p2pk_sigall_transaction_signature_succeeds label=melt_P2PK_SIG_ALL_valid\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_htlc_sigall_preimage_only_no_pubkeys_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "hnAeFmaURIBM2yB2BRrBJUGxLnHU73F_4MuL_MhW",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"2d76007ab96399629d21fc8b1ed913a042e1352e7b8ce04b3f767695b5f436f7\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02c1e6837f590b4d131bedc72df4e3a527e1fd328df75fee3d5dbf26789d2fd8f4",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"d870b2c4cb5a4c0df25939244b93fd8d393ea74d72f48abaedabd5eda3ad0a6c\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "025a1b9b53063f55b29d0c6c4230173a84d0c7cbef6d6ba3cbde1290957fcdb4c2"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_sigall_preimage_only_no_pubkeys_succeeds label=melt_HTLC_SIG_ALL_preimage_only_without_pubkeys\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, nil)
}

func TestCompatibility_melt_htlc_sigall_preimage_only_fails(t *testing.T) {
	payload := []byte(`{
  "quote": "Jo3mh0trrfmHUo642UhwCn4kbYSQghHjahCvVLst",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"80a2a75f5f332c2525c2c5bc4297bf7ec5339e958a88652f5758f83a251bbdde\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02395d6d4f521bdac6505c9a22034f5502d48aa42348787898ac9698a76557b67c\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "036d176147191ce1a451aad23d34a6e4d603a338df6b72628208e766a1072b8f65",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\"}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"4bb926f87e7e4c5897ccb9f5879b9b572210b20650b1d8d80f842f57e2695cc8\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02395d6d4f521bdac6505c9a22034f5502d48aa42348787898ac9698a76557b67c\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0239ce14376282f3a1aa1b78c5a1c5c46bc6baa1da2db01d16a400eb1dee8d0468"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_sigall_preimage_only_fails label=melt_HTLC_SIG_ALL_preimage_only\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_melt_htlc_sigall_sig_inputs_fail(t *testing.T) {
	payload := []byte(`{
  "quote": "Y6Un-XshIzyNdIzw_TR5GfMu3On6oG6kzV18yI_5",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"61e7557f73bb2663b61611928d35d23fe3b03ef183a2fe7de2f5a441698b4b62\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03c4591a678d053a56800a055d1a41653aa1e861851dffa841634f0a9c6400b4e2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "02377779237f25faa3c9a0922623645e6bebd52f0927fb6413d003ae3ed9d8326f",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"cede4670dd2f8feb3728846e42ff05ff412ad43fa685031819a43a1d6fb672e91b8fb7d61e8c6a533cfb8f37147c58df218d97afbbd179fcaac9c95ed88f9346\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"3ff5e4879f806bae8e39f590de15550e14dd275a2f4661cfa332bc7785e720fe\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"03c4591a678d053a56800a055d1a41653aa1e861851dffa841634f0a9c6400b4e2\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "03d897073a6000635d19f1c5e149022787df223322d3acbbb745a91850c156765b",
      "witness": "{\"signatures\":[\"a21c071fd81cc19685c63b6dd7fe8764452de064412707055758de893905afc9dabaddf9bba5b62e8e65918d6af921ce38028b2a8f45eae15946a507b606580d\"]}"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_sigall_sig_inputs_fail label=melt_HTLC_SIG_ALL_sig_inputs\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, ErrNotEnoughSignatures)
}

func TestCompatibility_melt_htlc_sigall_preimage_and_transaction_signature_succeeds(t *testing.T) {
	payload := []byte(`{
  "quote": "-0T4g9sRIXKyr8ZJI-4sfEuEGa1T8llOnVDTErUp",
  "inputs": [
    {
      "amount": 8,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"bd7543c88474d42669b12f9d35af1eaa716edbad91dab657b03342c8073009fb\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02338c5f020e19ae581e9e7cf10dcf5f7757f1534e8a1d10d3a0b2e2e7722a1e63\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "021c60653e4b11134fb3c7b65d1dbe10154ee5cb5d28c37969d7301921ffed7e4f",
      "witness": "{\"preimage\":\"4242424242424242424242424242424242424242424242424242424242424242\",\"signatures\":[\"cb037fc25dee074dec3ca5346761b6c62cbb8f9f3976351c9ab350be53e8345e8c58e8265245435dec72134fa433b279f3bb406bd1796751e0c87362d47d73fd\"]}"
    },
    {
      "amount": 4,
      "id": "0143cd3bb4a53bc6aeca481bb5ee707ea702939c83d9a86541be106c0e3dfcfe52",
      "secret": "[\"HTLC\",{\"nonce\":\"bdc66f2db4d3cf7aba92bb9d386bf9a93e1457b1997d51297fd9cc9fe706dcad\",\"data\":\"425ed4e4a36b30ea21b90e21c712c649e8214c29b7eaf68089d1039c6e55384c\",\"tags\":[[\"pubkeys\",\"02338c5f020e19ae581e9e7cf10dcf5f7757f1534e8a1d10d3a0b2e2e7722a1e63\"],[\"sigflag\",\"SIG_ALL\"]]}]",
      "C": "0375afff1d994f31b502fdd66027dcf7d2bfd0a46523fd02e2a17c288ebd005f13"
    }
  ],
  "outputs": null,
  "prefer_async": false
}
`)
	t.Logf("scenario=melt_htlc_sigall_preimage_and_transaction_signature_succeeds label=melt_HTLC_SIG_ALL_valid\n%s", payload)
	request1 := decodeMeltRequest(t, payload)
	err := validateMeltRequestForCompat(request1)
	assertCompatError(t, err, nil)
}
