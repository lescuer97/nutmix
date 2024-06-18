package lightning

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
)

// mppPaymentHashAndPreimage returns the payment hash and preimage to use for an
// MPP invoice.
func mppPaymentHashAndPreimage(d *invoicesrpc.AddInvoiceData) (*lntypes.Preimage,
	lntypes.Hash, error) {

	var (
		paymentPreimage *lntypes.Preimage
		paymentHash     lntypes.Hash
	)

	switch {

	// Only either preimage or hash can be set.
	case d.Preimage != nil && d.Hash != nil:
		return nil, lntypes.Hash{},
			errors.New("preimage and hash both set")

	// If no hash or preimage is given, generate a random preimage.
	case d.Preimage == nil && d.Hash == nil:
		paymentPreimage = &lntypes.Preimage{}
		if _, err := rand.Read(paymentPreimage[:]); err != nil {
			return nil, lntypes.Hash{}, err
		}
		paymentHash = paymentPreimage.Hash()

	// If just a hash is given, we create a hold invoice by setting the
	// preimage to unknown.
	case d.Preimage == nil && d.Hash != nil:
		paymentHash = *d.Hash

	// A specific preimage was supplied. Use that for the invoice.
	case d.Preimage != nil && d.Hash == nil:
		preimage := *d.Preimage
		paymentPreimage = &preimage
		paymentHash = d.Preimage.Hash()
	}

	return paymentPreimage, paymentHash, nil
}

func CreateMockInvoice(amountSats int64, description string, network chaincfg.Params) (string, error) {
	milsats, err := lnrpc.UnmarshallAmt(amountSats, 0)
	if err != nil {
		return "", fmt.Errorf("UnmarshallAmt: %w", err)
	}

	invoiceData := invoicesrpc.AddInvoiceData{
		Memo:     description,
		Value:    milsats,
		Preimage: nil,
		Expiry:   3600,
		Private:  false,
		Hash:     nil,
	}

	_, paymentHash, err := mppPaymentHashAndPreimage(&invoiceData)

	var options []func(*zpay32.Invoice)

	options = append(options, zpay32.Description(description))
	options = append(options, zpay32.Amount(milsats))
	options = append(options, zpay32.CLTVExpiry(64000))

	// Generate and set a random payment address for this invoice. If the
	// sender understands payment addresses, this can be used to avoid
	// intermediaries probing the receiver.
	var paymentAddr [32]byte
	if _, err := rand.Read(paymentAddr[:]); err != nil {
		return "", fmt.Errorf("paymentAddres Creation: %w", err)
	}
	options = append(options, zpay32.PaymentAddr(paymentAddr))

	creationTime := time.Now()
	payReq, err := zpay32.NewInvoice(&network, paymentHash, creationTime, options...)

	// Set our desired invoice features and add them to our list of options.
	var invoiceFeatures *lnwire.FeatureVector
	options = append(options, zpay32.Features(invoiceFeatures))

	if err != nil {
		return "", err

	}

	payReqString, err := payReq.Encode(zpay32.MessageSigner{
		SignCompact: func(msg []byte) ([]byte, error) {
			key, err := secp256k1.GeneratePrivateKey()

			if err != nil {
				return make([]byte, 0), fmt.Errorf("GeneratePrivateKey: %w ", err)
			}

			return ecdsa.SignCompact(key, msg, true), nil
		},
	})

	if err != nil {
		return "", fmt.Errorf("SignMessage: %w", err)
	}

	return payReqString, nil
}
