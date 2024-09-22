package lightning

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lescuer97/nutmix/api/cashu"
	cln_grpc "github.com/lescuer97/nutmix/internal/lightning/proto"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type CLNGRPCWallet struct {
	Network    chaincfg.Params
	grpcClient *grpc.ClientConn
	macaroon   string
}

func getTlsConfig(clientCert string, clientKey string, caCert string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return tlsConfig, fmt.Errorf("\nerror loading X.509 key pair: %v", err)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}
	if caCert != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(caCert))
		tlsConfig.RootCAs = pool
	}
	return tlsConfig, nil
}

func (l *CLNGRPCWallet) SetupGrpc(host string, caCert string, clientCert string, clientKey string, macaroon string) error {
	if host == "" {
		return fmt.Errorf("CLN_HOST not available")
	}

	if caCert == "" {
		return fmt.Errorf("CLN_CA_CERT not available")
	}
	if clientCert == "" {
		return fmt.Errorf("CLN_CLIENT_CERT not available")
	}

	if clientKey == "" {
		return fmt.Errorf("CLN_CLIENT_KEY not available")
	}

	tlsConfig, err := getTlsConfig(clientCert, clientKey, caCert)
	if err != nil {
		return fmt.Errorf("getTlsConfig(clientCert, clientKey, tlsCrt) %v", err)
	}

	certFile := credentials.NewTLS(tlsConfig)

	tlsDialOption := grpc.WithTransportCredentials(certFile)

	dialOpts := []grpc.DialOption{
		tlsDialOption,
	}

	clientConn, err := grpc.Dial(host, dialOpts...)

	if err != nil {
		return err
	}

	if macaroon == "" {
		return fmt.Errorf("LND_MACAROON_PATH not available")
	}

	l.macaroon = macaroon
	l.grpcClient = clientConn
	return nil
}

func (l *CLNGRPCWallet) clnGrpcPayInvoice(invoice string, feeReserve uint64, lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)
	client := cln_grpc.NewNodeClient(l.grpcClient)

	max_fee := cln_grpc.Amount{
		Msat: feeReserve * 1000,
	}
	sendRequest := cln_grpc.PayRequest{Bolt11: invoice, Maxfee: &max_fee}

	res, err := client.Pay(ctx, &sendRequest)

	if err != nil {
		return err
	}
	// res.
	if res.Status == cln_grpc.PayResponse_FAILED {
		return fmt.Errorf("Payment failed")
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaymentError = fmt.Errorf("")
	lightningResponse.PaidFeeSat = int64((res.AmountSentMsat.Msat - res.AmountMsat.Msat)) / 1000

	return nil

}
func (l *CLNGRPCWallet) clnGrpcPayPartialInvoice(invoice string,
	zpayInvoice *zpay32.Invoice,
	feeReserve uint64,
	amount_sat uint64,
	lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)
	client := cln_grpc.NewNodeClient(l.grpcClient)

	max_fee := cln_grpc.Amount{
		Msat: feeReserve * 1000,
	}

	partialSats := cln_grpc.Amount{
		Msat: amount_sat * 1000,
	}
	sendRequest := cln_grpc.PayRequest{Bolt11: invoice, Maxfee: &max_fee, PartialMsat: &partialSats}

	res, err := client.Pay(ctx, &sendRequest)

	if err != nil {
		return err
	}
	if res.Status == cln_grpc.PayResponse_FAILED {
		return fmt.Errorf("Payment failed")
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaymentError = fmt.Errorf("")
	lightningResponse.PaidFeeSat = int64((res.AmountSentMsat.Msat - res.AmountMsat.Msat)) / 1000

	return nil

}

func (l CLNGRPCWallet) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error) {
	var invoiceRes PaymentResponse
	if mpp {
		err := l.clnGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
		}
	} else {
		err := l.clnGrpcPayInvoice(invoice, feeReserve, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.LnbitsInvoiceRequest("POST", "/api/v1/payments", reqInvoice, &lnbitsInvoice) %w`, err)
		}
	}

	return invoiceRes, nil
}

func (l CLNGRPCWallet) CheckPayed(quote string) (cashu.ACTION_STATE, string, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	decodedHash, err := hex.DecodeString(quote)
	if err != nil {
		return cashu.UNPAID, "", fmt.Errorf("hex.DecodeString: %w. hash: %s", err, quote)
	}

	rhash := cln_grpc.ListpaysRequest{
		PaymentHash: decodedHash,
	}

	pays, err := client.ListPays(ctx, &rhash)

	if err != nil {
		return cashu.UNPAID, "", err
	}

	invoiceReq := cln_grpc.ListinvoicesRequest{
		PaymentHash: decodedHash,
	}
	invoices, err := client.ListInvoices(ctx, &invoiceReq)

	if err != nil {
		return cashu.UNPAID, "", err
	}

	fmt.Printf("\n pays %+v\n", pays)
	fmt.Printf("\n invice %+v\n", invoices)

	for _, pay := range pays.Pays {
		switch {
		case pay.Status == cln_grpc.ListpaysPays_COMPLETE:
			return cashu.PAID, hex.EncodeToString(pay.PaymentHash), nil

		case pay.Status == cln_grpc.ListpaysPays_FAILED:
			return cashu.UNPAID, hex.EncodeToString(pay.PaymentHash), nil

		}

	}
	for _, invoice := range invoices.Invoices {
		switch {
		case invoice.Status == cln_grpc.ListinvoicesInvoices_PAID:
			return cashu.PAID, hex.EncodeToString(invoice.PaymentHash), nil

		case invoice.Status == cln_grpc.ListinvoicesInvoices_UNPAID:
			return cashu.UNPAID, hex.EncodeToString(invoice.PaymentHash), nil

		}

	}
	return cashu.UNPAID, "", nil
}

func (l CLNGRPCWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	amount := cln_grpc.Amount{
		Msat: amount_sat * 1000,
	}

	queryRoutes := cln_grpc.GetrouteRequest{
		Id:         zpayInvoice.Destination.SerializeCompressed(),
		AmountMsat: &amount,
		Riskfactor: 10,
	}

	res, err := client.GetRoute(ctx, &queryRoutes)

	if err != nil {
		return 1, err
	}
	if res == nil {
		return 1, fmt.Errorf("No routes found")
	}

	if len(res.Route) == 0 {
		return 1, fmt.Errorf("No routes found")
	}

	fee := amount.Msat - *&res.Route[len(res.Route)-1].AmountMsat.Msat

	// turn to sats
	fee = fee / 1000

	return fee, nil
}

// type AmountAny struct {
//     Amount: *ccln_grpc.
//
// }

func (l CLNGRPCWallet) RequestInvoice(amount int64) (InvoiceResponse, error) {
	var response InvoiceResponse
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	amountCln := cln_grpc.Amount{
		Msat: uint64(amount) * 1000,
	}

	amountOrAllCln := cln_grpc.AmountOrAny_Amount{
		Amount: &amountCln,
	}

	req := cln_grpc.InvoiceRequest{
		AmountMsat: &cln_grpc.AmountOrAny{
			Value: &amountOrAllCln,
		},
		Label:       "",
		Description: "",
	}

	// Expiry time is 15 minutes
	res, err := client.Invoice(ctx, &req)

	if err != nil {
		return response, err
	}

	response.Rhash = hex.EncodeToString(res.PaymentHash)
	response.PaymentRequest = res.Bolt11

	return response, nil
}

func (l CLNGRPCWallet) WalletBalance() (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)
	client := cln_grpc.NewNodeClient(l.grpcClient)

	spent := false
	channelRequest := cln_grpc.ListfundsRequest{
		Spent: &spent,
	}

	balance, err := client.ListFunds(ctx, &channelRequest)

	if err != nil {
		return 0, err
	}

	fundsSat := uint64(0)

	for _, channel := range balance.Channels {

		fundsSat += channel.OurAmountMsat.Msat / 1000

	}

	return fundsSat, nil
}

func (f CLNGRPCWallet) LightningType() Backend {
	return CLNGRPC
}

func (f CLNGRPCWallet) GetNetwork() *chaincfg.Params {
	return &f.Network
}
func (f CLNGRPCWallet) ActiveMPP() bool {
	return true
}
