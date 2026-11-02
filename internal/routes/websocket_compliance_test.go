package routes

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	m "github.com/lescuer97/nutmix/internal/mint"
)

type websocketComplianceDB struct{ database.MintDB }

func (websocketComplianceDB) GetTx(context.Context) (pgx.Tx, error)  { return nil, nil }
func (websocketComplianceDB) Commit(context.Context, pgx.Tx) error   { return nil }
func (websocketComplianceDB) Rollback(context.Context, pgx.Tx) error { return nil }
func (websocketComplianceDB) GetProofsFromSecretCurve(pgx.Tx, []cashu.WrappedPublicKey) (cashu.Proofs, error) {
	return nil, nil
}

func newWebsocketComplianceServer(t *testing.T) (func() *websocket.Conn, *m.Mint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	observer := &m.Observer{Proofs: map[string][]m.ProofWatchChannel{}, MintQuote: map[string][]m.MintQuoteChannel{}, MeltQuote: map[string][]m.MeltQuoteChannel{}} //nolint:exhaustruct // Other observer maps are unrelated to websocket subscriptions.
	mint := &m.Mint{MintDB: websocketComplianceDB{}, Observer: observer}                                                                                            //nolint:exhaustruct // Only the database and observer are used by these route tests.
	router := gin.New()
	v1WebSocketRoute(router, mint)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"
	dial := func() *websocket.Conn {
		conn, response, err := websocket.DefaultDialer.Dial(url, nil)
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	return dial, mint
}

func wsRequest(id, sub string, filters []string) cashu.WsRequest {
	return cashu.WsRequest{JsonRpc: "2.0", Method: cashu.Subcribe, Id: idInt(id), Params: cashu.WebRequestParams{Payload: nil, Kind: cashu.ProofStateWs, SubId: sub, Filters: filters}}
}

func idInt(s string) int { var n int; _, _ = fmt.Sscanf(s, "%d", &n); return n }

func writeWSRequest(t *testing.T, conn *websocket.Conn, req cashu.WsRequest) {
	t.Helper()
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}
}

func readWSMessage(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("reading websocket message: %v", err)
	}
	return msg
}

func proofFilter() string {
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	return (cashu.WrappedPublicKey{PublicKey: pub}).ToHex()
}

func TestWebsocketMultipleSubscriptionsAndUnsubscribe(t *testing.T) {
	dial, mint := newWebsocketComplianceServer(t)
	conn := dial()
	filter := proofFilter()
	for i, id := range []string{"one", "two"} {
		req := wsRequest(fmt.Sprint(i+1), id, []string{filter})
		writeWSRequest(t, conn, req)
		if got := readWSMessage(t, conn); got["error"] != nil {
			t.Fatalf("subscription failed: %#v", got)
		}
		readWSMessage(t, conn) // initial snapshot
	}
	// Identically filtered subscriptions each receive their own update.
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	mint.Observer.SendProofsEvent(cashu.Proofs{{Y: cashu.WrappedPublicKey{PublicKey: pub}, C: cashu.WrappedPublicKey{PublicKey: nil}, Quote: nil, Id: "", Secret: "", Witness: "", State: "", Amount: 0, SeenAt: 0}})
	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		msg := readWSMessage(t, conn)
		params, ok := msg["params"].(map[string]any)
		if !ok {
			t.Fatalf("event params have unexpected type: %#v", msg["params"])
		}
		subID, ok := params["subId"].(string)
		if !ok {
			t.Fatalf("event subscription ID has unexpected type: %#v", params["subId"])
		}
		ids[subID] = true
	}
	if !ids["one"] || !ids["two"] {
		t.Fatalf("events missing subscriptions: %#v", ids)
	}
	writeWSRequest(t, conn, cashu.WsRequest{JsonRpc: "2.0", Method: cashu.Unsubcribe, Id: 3, Params: cashu.WebRequestParams{Payload: nil, Kind: "", SubId: "one", Filters: nil}})
	if readWSMessage(t, conn)["id"] != float64(3) {
		t.Fatal("unsubscribe not acknowledged")
	}
	mint.Observer.Lock()
	remaining := len(mint.Observer.Proofs[filter])
	mint.Observer.Unlock()
	if remaining != 1 {
		t.Fatalf("unsubscribe did not detach its watch: %d remain", remaining)
	}
}

func TestWebsocketConnectionScopedIDsAndLimits(t *testing.T) {
	dial, mint := newWebsocketComplianceServer(t)
	first, second := dial(), dial()
	filter := proofFilter()
	for _, c := range []*websocket.Conn{first, second} {
		writeWSRequest(t, c, wsRequest("1", "same", []string{filter}))
		if readWSMessage(t, c)["error"] != nil {
			t.Fatal("connection-scoped ID rejected")
		}
		readWSMessage(t, c)
	}
	// Fill to the per-connection subscription cap, verify rejection does not
	// close the socket, then release a slot and accept a replacement.
	for i := 1; i < maxSubscriptionsPerConnection; i++ {
		writeWSRequest(t, first, wsRequest(fmt.Sprint(i+1), fmt.Sprintf("s%d", i), []string{filter}))
		if readWSMessage(t, first)["error"] != nil {
			t.Fatalf("subscription %d rejected", i)
		}
		readWSMessage(t, first)
	}
	writeWSRequest(t, first, wsRequest("40", "overflow", []string{filter}))
	if readWSMessage(t, first)["error"] == nil {
		t.Fatal("subscription limit not enforced")
	}
	writeWSRequest(t, first, cashu.WsRequest{JsonRpc: "2.0", Method: cashu.Unsubcribe, Id: 41, Params: cashu.WebRequestParams{Payload: nil, Kind: "", SubId: "same", Filters: nil}})
	readWSMessage(t, first)
	writeWSRequest(t, first, wsRequest("42", "replacement", []string{filter}))
	if readWSMessage(t, first)["error"] != nil {
		t.Fatal("unsubscribe did not release subscription capacity")
	}
	readWSMessage(t, first)
	_ = mint
}

func TestWebsocketFailedSnapshotReleasesSubscription(t *testing.T) {
	dial, _ := newWebsocketComplianceServer(t)
	conn := dial()
	writeWSRequest(t, conn, wsRequest("1", "retry", []string{"not-a-public-key"}))
	if got := readWSMessage(t, conn); got["error"] == nil {
		t.Fatalf("invalid snapshot filter was accepted: %#v", got)
	}
	writeWSRequest(t, conn, wsRequest("2", "retry", []string{proofFilter()}))
	if got := readWSMessage(t, conn); got["error"] != nil {
		t.Fatalf("failed snapshot leaked its subscription: %#v", got)
	}
	readWSMessage(t, conn)
}

func TestWebsocketFilterLimitAndUnsubscribeRelease(t *testing.T) {
	dial, _ := newWebsocketComplianceServer(t)
	conn := dial()
	filters := make([]string, 0, maxFiltersPerConnection)
	for i := 1; i <= maxFiltersPerConnection; i++ {
		priv := make([]byte, 32)
		priv[30], priv[31] = byte(i>>8), byte(i)
		_, pub := btcec.PrivKeyFromBytes(priv)
		filters = append(filters, (cashu.WrappedPublicKey{PublicKey: pub}).ToHex())
	}
	for i := 0; i < len(filters); i += 6 {
		end := i + 6
		if end > len(filters) {
			end = len(filters)
		}
		batch := append([]string(nil), filters[i:end]...)
		if i == 0 {
			batch = append(batch, batch[0])
		}
		req := cashu.WsRequest{JsonRpc: "2.0", Method: cashu.Subcribe, Id: i + 1, Params: cashu.WebRequestParams{Payload: nil, Kind: cashu.ProofStateWs, SubId: fmt.Sprintf("batch-%d", i), Filters: batch}}
		writeWSRequest(t, conn, req)
		if got := readWSMessage(t, conn); got["error"] != nil {
			t.Fatalf("valid filter batch rejected: %#v", got)
		}
		for range batch[:len(batch)-boolInt(i == 0)] {
			readWSMessage(t, conn)
		}
	}
	rejected := wsRequest("90", "overflow", []string{filters[0]})
	writeWSRequest(t, conn, rejected)
	if got := readWSMessage(t, conn); got["error"] == nil {
		t.Fatal("filter limit was not enforced")
	}
	writeWSRequest(t, conn, cashu.WsRequest{JsonRpc: "2.0", Method: cashu.Unsubcribe, Id: 91, Params: cashu.WebRequestParams{Payload: nil, Kind: "", SubId: "batch-0", Filters: nil}})
	readWSMessage(t, conn)
	rejected.Params.SubId = "replacement"
	writeWSRequest(t, conn, rejected)
	if got := readWSMessage(t, conn); got["error"] != nil {
		t.Fatalf("unsubscribe did not release filter capacity: %#v", got)
	}
	readWSMessage(t, conn)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestWebsocketDisconnectDetachesOnlyThatConnection(t *testing.T) {
	dial, mint := newWebsocketComplianceServer(t)
	first, second := dial(), dial()
	filter := proofFilter()
	for _, conn := range []*websocket.Conn{first, second} {
		writeWSRequest(t, conn, wsRequest("1", "same", []string{filter}))
		if readWSMessage(t, conn)["error"] != nil {
			t.Fatal("subscription rejected")
		}
		readWSMessage(t, conn)
	}
	_ = first.Close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mint.Observer.Lock()
		remaining := len(mint.Observer.Proofs[filter])
		mint.Observer.Unlock()
		if remaining == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("disconnect left stale watch: %d remain", remaining)
		}
		time.Sleep(time.Millisecond)
	}
	_, pub := btcec.PrivKeyFromBytes([]byte{1})
	mint.Observer.SendProofsEvent(cashu.Proofs{{Y: cashu.WrappedPublicKey{PublicKey: pub}, C: cashu.WrappedPublicKey{PublicKey: nil}, Quote: nil, Id: "", Secret: "", Witness: "", State: "", Amount: 0, SeenAt: 0}})
	msg := readWSMessage(t, second)
	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatalf("event params have unexpected type: %#v", msg["params"])
	}
	got, ok := params["subId"].(string)
	if !ok || got != "same" {
		t.Fatalf("other connection lost subscription: %v", got)
	}
}
