package lightning

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
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
	feeReserve cashu.Amount,
	amount cashu.Amount,
	lightningResponse *PaymentResponse) error {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)
	client := cln_grpc.NewNodeClient(l.grpcClient)

	err := amount.To(cashu.Msat)
	if err != nil {
		return fmt.Errorf(`amount.To(cashu.Msat) %w`, err)
	}
	err = feeReserve.To(cashu.Msat)
	if err != nil {
		return fmt.Errorf(`feeReserve.To(cashu.Msat) %w`, err)
	}

	max_fee := cln_grpc.Amount{
		Msat: feeReserve.Amount,
	}

	partialSats := cln_grpc.Amount{
		Msat: amount.Amount,
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

func (l CLNGRPCWallet) PayInvoice(melt_quote cashu.MeltRequestDB, zpayInvoice *zpay32.Invoice, feeReserve uint64, mpp bool, amount cashu.Amount) (PaymentResponse, error) {
	var invoiceRes PaymentResponse

	hexHash := hex.EncodeToString(zpayInvoice.PaymentHash[:])

	// first check if invoice is already paid.
	status, _, _, err := l.CheckPayed(hexHash)
	if err != nil {
		return invoiceRes, fmt.Errorf(`l.CheckPayed(hexHash) %w`, err)
	}

	if status == SETTLED {
		return invoiceRes, ErrAlreadyPaid
	}
	if mpp {
		err := l.clnGrpcPayPartialInvoice(melt_quote.Request, zpayInvoice, cashu.Amount{Unit: cashu.Sat, Amount: feeReserve}, amount, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.lndGrpcPayPartialInvoice(invoice, zpayInvoice, feeReserve, amount_sat, &invoiceRes) %w`, err)
		}
	} else {
		err := l.clnGrpcPayInvoice(melt_quote.Request, feeReserve, &invoiceRes)
		if err != nil {
			return invoiceRes, fmt.Errorf(`l.clnGrpcPayInvoice(invoice, feeReserve, &invoiceRes) %w`, err)
		}
	}

	return invoiceRes, nil
}

func (l CLNGRPCWallet) CheckPayed(quote string) (PaymentStatus, string, uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)
	fee := uint64(0)

	decodedHash, err := hex.DecodeString(quote)
	if err != nil {
		return FAILED, "", fee, fmt.Errorf("hex.DecodeString: %w. hash: %s", err, quote)
	}

	rhash := cln_grpc.ListpaysRequest{
		PaymentHash: decodedHash,
	}

	pays, err := client.ListPays(ctx, &rhash)

	if err != nil {
		return FAILED, "", fee, err
	}

	for _, pay := range pays.Pays {
		switch {
		case pay.Status == cln_grpc.ListpaysPays_COMPLETE:
			fee := (pay.AmountSentMsat.Msat - pay.AmountMsat.Msat) / 1000
			return SETTLED, hex.EncodeToString(pay.PaymentHash), fee, nil
		case pay.Status == cln_grpc.ListpaysPays_PENDING:
			return PENDING, hex.EncodeToString(pay.PaymentHash), fee, nil
		case pay.Status == cln_grpc.ListpaysPays_FAILED:
			return FAILED, hex.EncodeToString(pay.PaymentHash), fee, nil

		}

	}
	return PENDING, "", fee, nil
}
func (l CLNGRPCWallet) CheckReceived(quote string) (PaymentStatus, string, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	decodedHash, err := hex.DecodeString(quote)
	if err != nil {
		return FAILED, "", fmt.Errorf("hex.DecodeString: %w. hash: %s", err, quote)
	}

	invoiceReq := cln_grpc.ListinvoicesRequest{
		PaymentHash: decodedHash,
	}

	invoices, err := client.ListInvoices(ctx, &invoiceReq)

	if err != nil {
		return FAILED, "", err
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

func (l CLNGRPCWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	_, _, _, err := l.CheckPayed(hex.EncodeToString(zpayInvoice.PaymentHash[:]))

	if err != nil {
		return 1, fmt.Errorf(`l.CheckPayed(invoice) %w`, err)
	}
	client := cln_grpc.NewNodeClient(l.grpcClient)

	err = amount.To(cashu.Msat)
	if err != nil {
		return 1, fmt.Errorf(`amount.To(cashu.Msat) %w`, err)
	}

	amountGrpc := cln_grpc.Amount{
		Msat: amount.Amount,
	}

	queryRoutes := cln_grpc.GetrouteRequest{
		Id:         zpayInvoice.Destination.SerializeCompressed(),
		AmountMsat: &amountGrpc,
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

	fee := amountGrpc.Msat - *&res.Route[len(res.Route)-1].AmountMsat.Msat

	// turn to sats
	fee = fee / 1000

	fee = GetFeeReserve(amount.Amount, fee)

	return fee, nil
}

func (l CLNGRPCWallet) RequestInvoice(amount cashu.Amount) (InvoiceResponse, error) {
	var response InvoiceResponse
	supported := l.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit). %w.", cashu.ErrUnitNotSupported)
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "macaroon", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	err := amount.To(cashu.Msat)
	if err != nil {
		return response, fmt.Errorf(`uuid.NewRandom() %w`, err)
	}
	amountCln := cln_grpc.Amount{
		Msat: amount.Amount,
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
func (f CLNGRPCWallet) VerifyUnitSupport(unit cashu.Unit) bool {
	if unit == cashu.Sat {
		return true
	} else {
		return false
	}
}
