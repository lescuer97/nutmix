package lightning

import (
	"bytes"
	"context"
	"encoding/hex"
	"math"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lescuer97/nutmix/api/cashu"
	cln_grpc "github.com/lescuer97/nutmix/internal/lightning/proto"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type clnRPCServer struct {
	cln_grpc.UnimplementedNodeServer
	xpay   func(*cln_grpc.XpayRequest) (*cln_grpc.XpayResponse, error)
	routes func(*cln_grpc.GetroutesRequest) (*cln_grpc.GetroutesResponse, error)
	info   func() (*cln_grpc.GetinfoResponse, error)
}

func (s clnRPCServer) Xpay(ctx context.Context, req *cln_grpc.XpayRequest) (*cln_grpc.XpayResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("rune")) != 1 || md.Get("rune")[0] != "test" {
		return nil, status.Error(codes.Unauthenticated, "missing rune")
	}
	return s.xpay(req)
}

func (s clnRPCServer) GetRoutes(ctx context.Context, req *cln_grpc.GetroutesRequest) (*cln_grpc.GetroutesResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("rune")) != 1 || md.Get("rune")[0] != "test" {
		return nil, status.Error(codes.Unauthenticated, "missing rune")
	}
	return s.routes(req)
}

func (s clnRPCServer) Getinfo(context.Context, *cln_grpc.GetinfoRequest) (*cln_grpc.GetinfoResponse, error) {
	return s.info()
}

func (s clnRPCServer) ListPays(context.Context, *cln_grpc.ListpaysRequest) (*cln_grpc.ListpaysResponse, error) {
	return &cln_grpc.ListpaysResponse{Pays: nil}, nil
}

func TestCLNXpay(t *testing.T) {
	for _, partial := range []bool{false, true} {
		for _, unit := range []cashu.Unit{cashu.Sat, cashu.Msat} {
			for _, outcome := range []string{"success", "rpc error", "missing amounts", "invalid amounts", "missing preimage"} {
				t.Run(unit.String()+"/"+map[bool]string{false: "full", true: "partial"}[partial]+"/"+outcome, func(t *testing.T) {
					preimage := bytes.Repeat([]byte{1}, 32)
					called := false
					//nolint:exhaustruct // Only Xpay is used by these payment helpers.
					server := clnRPCServer{
						xpay: func(req *cln_grpc.XpayRequest) (*cln_grpc.XpayResponse, error) {
							called = true
							if req.Invstring != "invoice" || req.Maxfee == nil || req.Maxfee.Msat != 5000 {
								t.Errorf("unexpected Xpay request: %v", req)
							}
							if partial {
								if req.PartialMsat == nil || req.PartialMsat.Msat != 100000 {
									t.Errorf("unexpected partial amount: %v", req.PartialMsat)
								}
							} else if req.PartialMsat != nil {
								t.Errorf("full payment has partial amount: %v", req.PartialMsat)
							}
							if outcome == "rpc error" {
								return nil, status.Error(codes.Unknown, "payment failed")
							}
							res := &cln_grpc.XpayResponse{ //nolint:exhaustruct
								PaymentPreimage: preimage,
								AmountMsat:      &cln_grpc.Amount{Msat: 100000},
								AmountSentMsat:  &cln_grpc.Amount{Msat: 102000},
							}
							switch outcome {
							case "missing amounts":
								res.AmountSentMsat = nil
							case "invalid amounts":
								res.AmountSentMsat.Msat = 99000
							case "missing preimage":
								res.PaymentPreimage = nil
							}
							return res, nil
						},
					}
					conn := newStatusTestConn(t, func(s *grpc.Server) { cln_grpc.RegisterNodeServer(s, server) })
					wallet := CLNGRPCWallet{grpcClient: conn, macaroon: "test"} //nolint:exhaustruct
					fee := cashu.NewAmount(unit, 5)
					amount := cashu.NewAmount(unit, 100)
					wantFee := cashu.NewAmount(unit, 2)
					if unit == cashu.Msat {
						fee.Amount = 5000
						amount.Amount = 100000
						wantFee.Amount = 2000
					}
					quote := cashu.MeltRequestDB{Request: "invoice", CheckingId: "checking"} //nolint:exhaustruct
					invoice := &zpay32.Invoice{PaymentHash: &[32]byte{}}                     //nolint:exhaustruct
					response, err := wallet.PayInvoice(quote, invoice, fee, partial, amount)
					if !called {
						t.Fatal("Xpay was not called")
					}
					if outcome != "success" {
						if err == nil || response.PaymentState == SETTLED {
							t.Fatalf("payment error = %v, state = %v", err, response.PaymentState)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if response.PaymentState != SETTLED || response.Preimage != hex.EncodeToString(preimage) || response.PaymentRequest != "invoice" || response.PaidFee != wantFee || response.CheckingId != quote.CheckingId {
						t.Fatalf("unexpected payment response: %+v", response)
					}
				})
			}
		}
	}
}

func TestCLNGetRoutes(t *testing.T) {
	_, destination := btcec.PrivKeyFromBytes([]byte{1})
	_, source := btcec.PrivKeyFromBytes([]byte{2})
	hash := [32]byte{3}
	for _, test := range []struct {
		name    string
		outcome string
		parts   [][2]uint64 // delivered, sent
		unit    cashu.Unit
		wantFee uint64
		mpp     bool
	}{
		{name: "single path", mpp: false, unit: cashu.Msat, parts: [][2]uint64{{100000, 102000}}, outcome: "success", wantFee: 2000},
		{name: "multiple parts", mpp: true, unit: cashu.Sat, parts: [][2]uint64{{40000, 42000}, {60000, 63000}}, outcome: "success", wantFee: 5},
		{name: "minimum reserve", mpp: false, unit: cashu.Sat, parts: [][2]uint64{{100000, 100100}}, outcome: "success", wantFee: 1},
		{name: "no routes", mpp: false, unit: cashu.Sat, parts: nil, outcome: "empty", wantFee: 0},
		{name: "missing path", mpp: false, unit: cashu.Sat, parts: [][2]uint64{{100000, 102000}}, outcome: "missing path", wantFee: 0},
		{name: "missing input", mpp: false, unit: cashu.Sat, parts: [][2]uint64{{100000, 102000}}, outcome: "missing input", wantFee: 0},
		{name: "invalid amounts", mpp: false, unit: cashu.Sat, parts: [][2]uint64{{100000, 99000}}, outcome: "invalid amounts", wantFee: 0},
		{name: "overflow", mpp: true, unit: cashu.Msat, parts: [][2]uint64{{1, math.MaxUint64}, {1, 3}}, outcome: "overflow", wantFee: 0},
		{name: "getroutes error", mpp: false, unit: cashu.Sat, parts: nil, outcome: "rpc error", wantFee: 0},
		{name: "getinfo error", mpp: false, unit: cashu.Sat, parts: nil, outcome: "info error", wantFee: 0},
		{name: "invalid CLTV", mpp: false, unit: cashu.Sat, parts: nil, outcome: "invalid CLTV", wantFee: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			invoice := &zpay32.Invoice{Destination: destination, PaymentHash: &hash} //nolint:exhaustruct
			zpay32.CLTVExpiry(42)(invoice)
			if test.outcome == "invalid CLTV" {
				zpay32.CLTVExpiry(math.MaxUint64)(invoice)
			}
			if test.mpp {
				invoice.Features = lnwire.NewFeatureVector(lnwire.NewRawFeatureVector(lnwire.MPPOptional), lnwire.Features)
			}
			called := false
			//nolint:exhaustruct // Only fee-query RPCs are used here.
			server := clnRPCServer{
				info: func() (*cln_grpc.GetinfoResponse, error) {
					if test.outcome == "info error" {
						return nil, status.Error(codes.Unavailable, "offline")
					}
					return &cln_grpc.GetinfoResponse{Id: source.SerializeCompressed()}, nil //nolint:exhaustruct
				},
				routes: func(req *cln_grpc.GetroutesRequest) (*cln_grpc.GetroutesResponse, error) {
					called = true
					if !bytes.Equal(req.Source, source.SerializeCompressed()) || !bytes.Equal(req.Destination, destination.SerializeCompressed()) || req.AmountMsat.GetMsat() != 100000 || req.FinalCltv != 42 {
						t.Errorf("unexpected GetRoutes request: %v", req)
					}
					if len(req.Layers) != 2 || req.Layers[0] != "auto.localchans" || req.Layers[1] != "auto.sourcefree" || req.MaxfeeMsat.GetMsat() != math.MaxUint64-100000 {
						t.Errorf("unexpected routing constraints: %v", req)
					}
					if test.mpp && req.Maxparts != nil || !test.mpp && (req.Maxparts == nil || *req.Maxparts != 1) {
						t.Errorf("unexpected Maxparts: %v", req.Maxparts)
					}
					if test.outcome == "rpc error" {
						return nil, status.Error(codes.Unknown, "no route")
					}
					res := &cln_grpc.GetroutesResponse{}
					for _, part := range test.parts {
						res.Routes = append(res.Routes, &cln_grpc.GetroutesRoutes{ //nolint:exhaustruct
							AmountMsat: &cln_grpc.Amount{Msat: part[0]},
							Path: []*cln_grpc.GetroutesRoutesPath{
								{AmountInMsat: &cln_grpc.Amount{Msat: part[1]}}, //nolint:exhaustruct
							},
						})
					}
					if test.outcome == "missing path" {
						res.Routes[0].Path = nil
					}
					if test.outcome == "missing input" {
						res.Routes[0].Path[0].AmountInMsat = nil
					}
					return res, nil
				},
			}
			conn := newStatusTestConn(t, func(s *grpc.Server) { cln_grpc.RegisterNodeServer(s, server) })
			wallet := CLNGRPCWallet{grpcClient: conn, macaroon: "test"} //nolint:exhaustruct
			amount := cashu.NewAmount(test.unit, 100)
			if test.unit == cashu.Msat {
				amount.Amount = 100000
			}
			res, err := wallet.QueryFees("invoice", invoice, false, amount)
			if test.outcome != "success" {
				if err == nil {
					t.Fatal("expected fee-query error")
				}
				if (test.outcome == "info error" || test.outcome == "invalid CLTV") && called {
					t.Fatal("GetRoutes called after invalid request or failed Getinfo")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !called || res.Fees != cashu.NewAmount(test.unit, test.wantFee) || res.AmountToSend != amount || res.CheckingId != hex.EncodeToString(hash[:]) {
				t.Fatalf("unexpected fee response: %+v (GetRoutes called: %v)", res, called)
			}
		})
	}
}
