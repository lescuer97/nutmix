package mint

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func TestValidateMintIssuanceAuthRejectsInvalidSignature(t *testing.T) {
	quoteKey, _ := btcec.PrivKeyFromBytes([]byte{1})
	signingKey, _ := btcec.PrivKeyFromBytes([]byte{2})
	//nolint:exhaustruct // Signature is set below; only the signed fields matter here.
	request := cashu.PostMintBolt11Request{
		Quote: "quote",
		//nolint:exhaustruct // Signature is set below; only the signed fields matter here.
		Outputs: cashu.BlindedMessages{{
			Amount: 1,
			//nolint:exhaustruct // The blinded point is the only output field used for signing.
			B_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{3}).PubKey()},
		}},
	}
	message, err := cashu.MintQuoteSignatureMessage(request.Quote, request.Outputs)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	request.Signature, err = schnorr.Sign(signingKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}

	var mint Mint
	//nolint:exhaustruct // Only the quote's authorization key affects this validation.
	err = mint.validateMintIssuanceAuth(request, cashu.MintRequestDB{
		//nolint:exhaustruct // Only the quote's authorization key affects this validation.
		Pubkey: cashu.WrappedPublicKey{PublicKey: quoteKey.PubKey()},
	})
	if !errors.Is(err, cashu.ErrMintQuoteNoValidSignature) {
		t.Fatalf("expected ErrMintQuoteNoValidSignature, got %v", err)
	}
	code, _ := utils.ParseErrorToCashuErrorCode(err)
	if code != cashu.MINT_QUOTE_INVALID_SIG {
		t.Fatalf("expected MINT_QUOTE_INVALID_SIG, got %v", code)
	}
}
