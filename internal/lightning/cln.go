package lightning

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
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
			return tlsConfig, fmt.Errorf("\n error loading X.509 key pair: %v", err)
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

	if res.Status == cln_grpc.PayResponse_FAILED {
		return fmt.Errorf("Payment failed")
	}
	switch res.Status {
	case cln_grpc.PayResponse_FAILED:
		lightningResponse.PaymentState = FAILED
		return fmt.Errorf("Payment failed")
	case cln_grpc.PayResponse_PENDING:
		lightningResponse.PaymentState = PENDING
		return nil
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.PaymentState = SETTLED
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaidFeeSat = int64((res.AmountSentMsat.Msat - res.AmountMsat.Msat)) / 1000

	return nil

}
func (l *CLNGRPCWallet) clnGrpcPayPartialInvoice(invoice string,
	_ *zpay32.Invoice,
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
	switch res.Status {
	case cln_grpc.PayResponse_FAILED:
		lightningResponse.PaymentState = FAILED
		return fmt.Errorf("Payment failed")
	case cln_grpc.PayResponse_PENDING:
		lightningResponse.PaymentState = PENDING
		return nil
	}

	lightningResponse.PaymentRequest = invoice
	lightningResponse.PaymentState = SETTLED
	lightningResponse.Preimage = hex.EncodeToString(res.PaymentPreimage)
	lightningResponse.PaidFeeSat = int64((res.AmountSentMsat.Msat - res.AmountMsat.Msat)) / 1000

	return nil

}

func (l CLNGRPCWallet) PayInvoice(invoice string, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount_sat uint64) (PaymentResponse, error) {
	var invoiceRes PaymentResponse

	hexHash := hex.EncodeToString(zpayInvoice.PaymentHash[:])

	// first check if invoice is already paid.
	status, _, err := l.CheckPayed(hexHash)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.CheckPayed(hexHash) %w`, err)
	}

	if status == SETTLED {
		return invoiceRes, ErrAlreadyPaid
	}
	if mpp {
		err := l.clnGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
		}
	} else {
		err := l.clnGrpcPayInvoice(invoice, feeReserve, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.clnGrpcPayInvoice(invoice, feeReserve, &invoiceRes) %w`, err)
		}
	}

	return invoiceRes, nil
}

func (l CLNGRPCWallet) CheckPayed(quote string) (PaymentStatus, string, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	decodedHash, err := hex.DecodeString(quote)
	if err != nil {
		return FAILED, "", fmt.Errorf("hex.DecodeString: %w. hash: %s", err, quote)
	}

	rhash := cln_grpc.ListpaysRequest{
		PaymentHash: decodedHash,
	}

	pays, err := client.ListPays(ctx, &rhash)

	if err != nil {
		return FAILED, "", err
	}

	invoiceReq := cln_grpc.ListinvoicesRequest{
		PaymentHash: decodedHash,
	}
	invoices, err := client.ListInvoices(ctx, &invoiceReq)

	if err != nil {
		return FAILED, "", err
	}

	for _, pay := range pays.Pays {
		switch {
		case pay.Status == cln_grpc.ListpaysPays_COMPLETE:
			return SETTLED, hex.EncodeToString(pay.PaymentHash), nil
		case pay.Status == cln_grpc.ListpaysPays_PENDING:
			return PENDING, hex.EncodeToString(pay.PaymentHash), nil
		case pay.Status == cln_grpc.ListpaysPays_FAILED:
			return SETTLED, hex.EncodeToString(pay.PaymentHash), nil

		}

	}
	for _, invoice := range invoices.Invoices {
		switch {
		case invoice.Status == cln_grpc.ListinvoicesInvoices_PAID:
			return SETTLED, hex.EncodeToString(invoice.PaymentHash), nil

		case invoice.Status == cln_grpc.ListinvoicesInvoices_UNPAID:
			return PENDING, hex.EncodeToString(invoice.PaymentHash), nil

		}

	}
	return PENDING, "", nil
}

func (l CLNGRPCWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount_sat uint64) (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	_, _, err := l.CheckPayed(hex.EncodeToString(zpayInvoice.PaymentHash[:]))

	if err != nil {
		return 1, fmt.Errorf(`l.CheckPayed(invoice) %w`, err)
	}
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
	randUuid, err := uuid.NewRandom()

	if err != nil {
		return response, fmt.Errorf(`uuid.NewRandom() %w`, err)
	}

	req := cln_grpc.InvoiceRequest{
		AmountMsat: &cln_grpc.AmountOrAny{
			Value: &amountOrAllCln,
		},
		Label:       randUuid.String(),
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

	fundsMSat := uint64(0)

	for _, channel := range balance.Channels {

		fundsMSat += channel.OurAmountMsat.Msat

	}

	return fundsMSat, nil
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
