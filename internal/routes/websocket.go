package routes

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
)

var ErrAlreadySubscribed = errors.New("filter already subscribed")

const (
	maxSubscriptionsPerConnection = 32
	maxFiltersPerConnection       = 128
)

type subscription struct {
	done    chan struct{}
	stopped chan struct{}
	owner   string
	filters int
}

func checkOrigin(r *http.Request) bool { return true }

func v1WebSocketRoute(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")
	//nolint:exhaustruct
	upgrader := websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: checkOrigin}
	v1.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Warn("websocket upgrade failed", slog.Any("error", err))
			return
		}
		connContext, cancelConn := context.WithCancel(context.WithoutCancel(c.Request.Context()))
		conn.SetReadLimit(1024)
		var writeMu sync.Mutex
		send := func(v any) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			return m.SendJson(conn, v)
		}
		subs := make(map[string]*subscription)
		filterCount := 0
		stop := func(id string) {
			s, ok := subs[id]
			if !ok {
				return
			}
			mint.Observer.RemoveWatch(s.owner)
			close(s.done)
			<-s.stopped
			delete(subs, id)
			filterCount -= s.filters
		}
		defer func() {
			cancelConn()
			_ = conn.Close()
			for id := range subs {
				stop(id)
			}
		}()
		for {
			var req cashu.WsRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.JsonRpc != "2.0" {
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32600, Message: "Invalid Request"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			if req.Method == cashu.Unsubcribe {
				stop(req.Params.SubId)
				if send(cashu.WsResponse{JsonRpc: "2.0", Id: req.Id, Result: cashu.WsResponseResult{Status: "OK", SubId: req.Params.SubId}}) != nil {
					return
				}
				continue
			}
			if req.Method != cashu.Subcribe {
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32601, Message: "Method not found"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			id := req.Params.SubId
			if id == "" || len(req.Params.Filters) == 0 {
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32602, Message: "Invalid params"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			if _, ok := subs[id]; ok || len(subs) >= maxSubscriptionsPerConnection {
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32602, Message: "Subscription limit exceeded"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			switch req.Params.Kind {
			case cashu.ProofStateWs, cashu.Bolt11MintQuote, cashu.Bolt11MeltQuote:
			default:
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32602, Message: "Invalid params"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			filters := make([]string, 0, len(req.Params.Filters))
			seen := make(map[string]bool, len(req.Params.Filters))
			invalid := false
			for _, f := range req.Params.Filters {
				if f == "" {
					invalid = true
					continue
				}
				if req.Params.Kind == cashu.ProofStateWs {
					f = strings.ToLower(f)
				}
				if !seen[f] {
					seen[f] = true
					filters = append(filters, f)
				}
			}
			if invalid || len(filters) == 0 || len(filters) > maxFiltersPerConnection-filterCount {
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32602, Message: "Invalid params or filter limit exceeded"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			req.Params.Filters = filters
			owner := fmt.Sprintf("%p:%s", conn, id)
			var p chan cashu.Proof
			var mi chan cashu.MintRequestDB
			var me chan cashu.MeltRequestDB
			switch req.Params.Kind {
			case cashu.ProofStateWs:
				p = make(chan cashu.Proof, 64)
				for _, f := range filters {
					mint.Observer.AddProofWatch(f, m.ProofWatchChannel{Channel: p, SubId: owner})
				}
			case cashu.Bolt11MintQuote:
				mi = make(chan cashu.MintRequestDB, 64)
				for _, f := range filters {
					mint.Observer.AddMintWatch(f, m.MintQuoteChannel{Channel: mi, SubId: owner})
				}
			case cashu.Bolt11MeltQuote:
				me = make(chan cashu.MeltRequestDB, 64)
				for _, f := range filters {
					mint.Observer.AddMeltWatch(f, m.MeltQuoteChannel{Channel: me, SubId: owner})
				}
			}
			s := &subscription{owner: owner, done: make(chan struct{}), stopped: make(chan struct{}), filters: len(filters)}
			subs[id] = s
			filterCount += len(filters)
			ready := make(chan struct{})
			go func(id string, s *subscription) {
				defer close(s.stopped)
				select {
				case <-s.done:
					return
				case <-ready:
				}
				for {
					select {
					case <-s.done:
						return
					case v := <-p:
						if send(cashu.WsNotification{JsonRpc: "2.0", Method: cashu.Subcribe, Params: cashu.WebRequestParams{Payload: cashu.CheckState{Y: v.Y, State: v.State, Witness: &v.Witness}, Kind: "", SubId: id, Filters: nil}, Id: 0}) != nil {
							_ = conn.Close()
							return
						}
					case v := <-mi:
						if send(cashu.WsNotification{JsonRpc: "2.0", Method: cashu.Subcribe, Params: cashu.WebRequestParams{Payload: v.PostMintQuoteBolt11Response(), Kind: "", SubId: id, Filters: nil}, Id: 0}) != nil {
							_ = conn.Close()
							return
						}
					case v := <-me:
						if send(cashu.WsNotification{JsonRpc: "2.0", Method: cashu.Subcribe, Params: cashu.WebRequestParams{Payload: v.GetPostMeltQuoteResponse(), Kind: "", SubId: id, Filters: nil}, Id: 0}) != nil {
							_ = conn.Close()
							return
						}
					}
				}
			}(id, s)
			ctx, cancel := context.WithTimeout(connContext, 15*time.Second)
			payloads, err := snapshotPayloads(ctx, req, mint)
			cancel()
			if err != nil {
				stop(id)
				if send(cashu.WsError{JsonRpc: "2.0", Error: cashu.ErrorMsg{Code: -32602, Message: "Invalid params"}, Id: req.Id}) != nil {
					return
				}
				continue
			}
			if send(cashu.WsResponse{JsonRpc: "2.0", Id: req.Id, Result: cashu.WsResponseResult{Status: "OK", SubId: id}}) != nil {
				return
			}
			for _, payload := range payloads {
				if send(cashu.WsNotification{JsonRpc: "2.0", Method: cashu.Subcribe, Params: cashu.WebRequestParams{Payload: payload, Kind: "", SubId: id, Filters: nil}, Id: 0}) != nil {
					return
				}
			}
			close(ready)
		}
	})
}

func snapshotPayloads(ctx context.Context, req cashu.WsRequest, mint *m.Mint) ([]any, error) {
	var out []any
	seen := map[string]bool{}
	for _, f := range req.Params.Filters {
		if f == "" {
			return nil, fmt.Errorf("empty filter")
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		switch req.Params.Kind {
		case cashu.Bolt11MintQuote:
			v, err := mint.RefreshMintQuoteStatus(ctx, f, m.Bolt11)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		case cashu.Bolt11MeltQuote:
			v, err := mint.RefreshMeltQuoteState(ctx, f)
			if err != nil {
				return nil, err
			}
			out = append(out, v.GetPostMeltQuoteResponse())
		case cashu.ProofStateWs:
			b, err := hex.DecodeString(f)
			if err != nil {
				return nil, err
			}
			key, err := btcec.ParsePubKey(b)
			if err != nil {
				return nil, err
			}
			states, err := m.CheckProofState(ctx, mint, []cashu.WrappedPublicKey{{PublicKey: key}})
			if err != nil {
				return nil, err
			}
			if len(states) > 0 {
				out = append(out, states[0])
			}
		}
	}
	return out, nil
}
