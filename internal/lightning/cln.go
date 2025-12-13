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

	clientConn, err := grpc.NewClient(host, dialOpts...)

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
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)
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
		return fmt.Errorf("payment failed")
	}
	switch res.Status {
	case cln_grpc.PayResponse_FAILED:
		lightningResponse.PaymentState = FAILED
		return fmt.Errorf("payment failed")
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
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)
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
		return fmt.Errorf("payment failed")
	}
	switch res.Status {
	case cln_grpc.PayResponse_FAILED:
		lightningResponse.PaymentState = FAILED
		return fmt.Errorf("payment failed")
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

	// first check if invoice is already paid.
	status, _, _, err := l.CheckPayed(melt_quote.Quote, zpayInvoice, melt_quote.CheckingId)
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
	invoiceRes.CheckingId = melt_quote.CheckingId

	return invoiceRes, nil
}

func (l CLNGRPCWallet) CheckPayed(quote string, invoice *zpay32.Invoice, checkingId string) (PaymentStatus, string, uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)
	fee := uint64(0)

	rhash := cln_grpc.ListpaysRequest{
		PaymentHash: invoice.PaymentHash[:],
	}

	pays, err := client.ListPays(ctx, &rhash)

	if err != nil {
		return FAILED, "", fee, err
	}

	for _, pay := range pays.Pays {
		switch pay.Status {
		case cln_grpc.ListpaysPays_COMPLETE:
			fee := (pay.AmountSentMsat.Msat - pay.AmountMsat.Msat) / 1000
			return SETTLED, hex.EncodeToString(pay.PaymentHash), fee, nil
		case cln_grpc.ListpaysPays_PENDING:
			return PENDING, hex.EncodeToString(pay.PaymentHash), fee, nil
		case cln_grpc.ListpaysPays_FAILED:
			return FAILED, hex.EncodeToString(pay.PaymentHash), fee, nil

		}

	}
	return PENDING, "", fee, nil
}
func (l CLNGRPCWallet) CheckReceived(quote cashu.MintRequestDB, invoice *zpay32.Invoice) (PaymentStatus, string, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)

	client := cln_grpc.NewNodeClient(l.grpcClient)

	invoiceReq := cln_grpc.ListinvoicesRequest{
		PaymentHash: invoice.PaymentHash[:],
	}

	invoices, err := client.ListInvoices(ctx, &invoiceReq)

	if err != nil {
		return FAILED, "", err
	}

	for _, invoice := range invoices.Invoices {
		switch invoice.Status {
		case cln_grpc.ListinvoicesInvoices_PAID:
			return SETTLED, hex.EncodeToString(invoice.PaymentHash), nil

		case cln_grpc.ListinvoicesInvoices_UNPAID:
			return PENDING, hex.EncodeToString(invoice.PaymentHash), nil

		}

	}
	return PENDING, "", nil
}

func (l CLNGRPCWallet) QueryFees(invoice string, zpayInvoice *zpay32.Invoice, mpp bool, amount cashu.Amount) (FeesResponse, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)

	feesResponse := FeesResponse{}
	_, _, _, err := l.CheckPayed("", zpayInvoice, "")

	if err != nil {
		return feesResponse, fmt.Errorf(`l.CheckPayed(invoice) %w`, err)
	}
	client := cln_grpc.NewNodeClient(l.grpcClient)

	err = amount.To(cashu.Msat)
	if err != nil {
		return feesResponse, fmt.Errorf(`amount.To(cashu.Msat) %w`, err)
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
		return feesResponse, err
	}
	if res == nil {
		return feesResponse, fmt.Errorf("no routes found")
	}

	if len(res.Route) == 0 {
		return feesResponse, fmt.Errorf("no routes found")
	}

	fee := amountGrpc.Msat - res.Route[len(res.Route)-1].AmountMsat.Msat

	// turn to sats
	fee = fee / 1000

	fee = GetFeeReserve(amount.Amount, fee)

	hash := zpayInvoice.PaymentHash[:]

	feesResponse.Fees.Amount = fee
	feesResponse.AmountToSend.Amount = amount.Amount
	feesResponse.CheckingId = hex.EncodeToString(hash)

	return feesResponse, nil
}

func (l CLNGRPCWallet) RequestInvoice(quote cashu.MintRequestDB, amount cashu.Amount) (InvoiceResponse, error) {
	var response InvoiceResponse
	supported := l.VerifyUnitSupport(amount.Unit)
	if !supported {
		return response, fmt.Errorf("l.VerifyUnitSupport(amount.Unit): %w", cashu.ErrUnitNotSupported)
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)

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
	if quote.Description != nil {
		req.Description = *quote.Description
	}

	// Expiry time is 15 minutes
	res, err := client.Invoice(ctx, &req)

	if err != nil {
		return response, err
	}

	response.Rhash = hex.EncodeToString(res.PaymentHash)
	response.CheckingId = hex.EncodeToString(res.PaymentHash)
	response.PaymentRequest = res.Bolt11

	return response, nil
}

func (l CLNGRPCWallet) WalletBalance() (uint64, error) {
	ctx := metadata.AppendToOutgoingContext(context.Background(), "rune", l.macaroon)
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
func (f CLNGRPCWallet) DescriptionSupport() bool {
	return true
}
