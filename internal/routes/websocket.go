package routes

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

var ErrAlreadySubscribed = errors.New("Filter already subscribed")

// type of subscription: map[filter]IdOfSubscription
type ActiveSubs map[cashu.SubscriptionKind]map[string]string

type WalletSubscription struct {
	Subscriptions ActiveSubs
	sync.Mutex
}

func (w *WalletSubscription) Subscribe(kind cashu.SubscriptionKind, filters []string, subId string) error {
	w.Lock()
	for i := 0; i < len(filters); i++ {
		_, kindExists := w.Subscriptions[kind]

		if kindExists {
			_, filterExists := w.Subscriptions[kind][filters[i]]
			if filterExists {
				return ErrAlreadySubscribed
			}
		} else {
			w.Subscriptions[kind] = make(map[string]string)

		}

		w.Subscriptions[kind][filters[i]] = subId
	}

	w.Unlock()
	return nil
}

func (w *WalletSubscription) Unsubcribe(subId string) {
	w.Lock()

	for kind, filters := range w.Subscriptions {
		for filter, id := range filters {
			if subId == id {
				delete(w.Subscriptions[kind], filter)
			}
		}
	}
	w.Unlock()
}
func checkOrigin(r *http.Request) bool {
	return true
}

func v1WebSocketRoute(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     checkOrigin,
	}

	v1.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Warn("upgrader.Upgrade(c.Writer, c.Request, nil)", slog.String(utils.LogExtraInfo, err.Error()))
			return
		}
		defer conn.Close()

		var request cashu.WsRequest

		err = conn.ReadJSON(&request)
		if err != nil {
			return
		}

		activeSubs := WalletSubscription{
			Subscriptions: make(ActiveSubs),
		}

		// parse request check if subscription or unsubscribe
		err = handleWSRequest(request, &activeSubs)

		if err != nil {
			if errors.Is(err, ErrAlreadySubscribed) {
				errMsg := cashu.WsError{
					JsonRpc: "2.0",
					Id:      request.Id,
					Error: cashu.ErrorMsg{
						Code:    cashu.UNKNOWN,
						Message: "Already subscribed to filter",
					},
				}
				err = m.SendJson(conn, errMsg)
			}
			logger.Error("Error on creating websocket %+v", slog.String(utils.LogExtraInfo, err.Error()))
			return
		}

		logger.Debug(fmt.Sprintf("New request %+v", request))
		// confirm subscription or unsubscribe
		response := cashu.WsResponse{
			JsonRpc: "2.0",
			Id:      request.Id,
			Result: cashu.WsResponseResult{
				Status: "OK",
				SubId:  request.Params.SubId,
			},
		}

		err = m.SendJson(conn, response)
		if err != nil {
			logger.Warn("m.SendJson(conn, response)", slog.String(utils.LogExtraInfo, err.Error()))
			return
		}

		listining := make(chan string)
		writing := make(chan string)

		go ListenToIncommingMessage(&activeSubs, conn, listining)

		go CheckingForSubsUpdates(&activeSubs, mint, conn, writing)
		select {
		case listenInfo := <-listining:
			logger.Warn("go ListenToIncommingMessage(&activeSubs, conn, listining).", slog.String(utils.LogExtraInfo, listenInfo))

		case writeInfo := <-writing:
			logger.Warn("go CheckingForSubsUpdates(&activeSubs, mint, conn, writing).", slog.String(utils.LogExtraInfo, writeInfo))

		}

	})
}

func handleWSRequest(request cashu.WsRequest, subs *WalletSubscription) error {
	switch request.Method {
	case cashu.Subcribe:
		err := subs.Subscribe(request.Params.Kind, request.Params.Filters, request.Params.SubId)
		if err != nil {
			return err
		}

	case cashu.Unsubcribe:
		subs.Unsubcribe(request.Params.SubId)
	}
	return nil
}

func ListenToIncommingMessage(subs *WalletSubscription, conn *websocket.Conn, ch chan string) {
	for {
		var request cashu.WsRequest
		err := conn.ReadJSON(&request)
		if err != nil {
			ch <- fmt.Errorf("conn.ReadJSON(&request).%w", err).Error()
			return
		}

		err = handleWSRequest(request, subs)
		if err != nil {
			ch <- fmt.Errorf("handleWSRequest(request, subs) %w", err).Error()
			return
		}
		response := cashu.WsResponse{
			JsonRpc: "2.0",
			Id:      request.Id,
			Result: cashu.WsResponseResult{
				Status: "OK",
				SubId:  request.Params.SubId,
			},
		}

		err = m.SendJson(conn, response)
		if err != nil {
			ch <- fmt.Errorf("m.SendJson(conn, response) %w", err).Error()
			return
		}
	}
}

func CheckingForSubsUpdates(subs *WalletSubscription, mint *m.Mint, conn *websocket.Conn, ch chan string) error {

	alreadyCheckedFilter := make(map[string]any)
	for {
		for kind, filters := range subs.Subscriptions {
			for filter, subId := range filters {
				// check if a new stored notif has already been seen and if no send a status update and store state
				value, exists := alreadyCheckedFilter[filter]

				statusNotif := cashu.WsNotification{
					JsonRpc: "2.0",
					Method:  cashu.Subcribe,
					Params: cashu.WebRequestParams{
						SubId: subId,
					},
				}

				switch kind {
				case cashu.Bolt11MintQuote:
					mintState, err := m.CheckMintRequest(mint, filter)
					if err != nil {
						return fmt.Errorf("m.CheckMintRequest(mint, filter). %w", err)
					}
					statusNotif.Params.Payload = mintState
					if exists {
						if value.(cashu.PostMintQuoteBolt11Response).State != mintState.State {
							alreadyCheckedFilter[filter] = mintState
							err := m.SendJson(conn, statusNotif)
							if err != nil {
								return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
							}
						}
					} else {
						alreadyCheckedFilter[filter] = mintState
						err := m.SendJson(conn, statusNotif)
						if err != nil {
							return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
						}
					}
				case cashu.Bolt11MeltQuote:
					meltState, err := m.CheckMeltRequest(mint, filter)
					if err != nil {
						return fmt.Errorf("m.CheckMeltRequest(mint, filter). %w", err)
					}

					statusNotif.Params.Payload = meltState
					if exists {

						if value.(cashu.PostMeltQuoteBolt11Response).State != meltState.State {
							alreadyCheckedFilter[filter] = meltState
							err := m.SendJson(conn, statusNotif)
							if err != nil {
								return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
							}
						}
					} else {
						alreadyCheckedFilter[filter] = meltState
						err := m.SendJson(conn, statusNotif)
						if err != nil {
							return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
						}
					}

				case cashu.ProofStateWs:
					proofsState, err := m.CheckProofState(mint, []string{filter})
					if err != nil {
						return fmt.Errorf("m.CheckProofState(mint, []string{filter}). %w", err)
					}
					// check for subscription and if the state changed
					if exists && len(proofsState) > 0 {
						if value.(cashu.CheckState).State != proofsState[0].State {
							statusNotif.Params.Payload = proofsState[0]

							alreadyCheckedFilter[filter] = proofsState[0]
							err := m.SendJson(conn, statusNotif)
							if err != nil {
								return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
							}
						}
					} else {
						statusNotif.Params.Payload = proofsState[0]
						alreadyCheckedFilter[filter] = proofsState[0]
						err := m.SendJson(conn, statusNotif)
						if err != nil {
							return fmt.Errorf("m.SendJson(conn, statusNotif). %w", err)
						}
					}
				}

			}
			time.Sleep(2 * time.Second)
		}
	}
}

func CheckStatusesOfSubscription(subKind cashu.SubscriptionKind, filters []string, pool *pgxpool.Pool, mint *m.Mint, ch chan string) ([]cashu.PostMintQuoteBolt11Response, []cashu.CheckState, error) {
	var mintQuote []cashu.PostMintQuoteBolt11Response
	var proofsState []cashu.CheckState
	switch subKind {
	case cashu.Bolt11MintQuote:
		for _, v := range filters {
			quote, err := m.CheckMintRequest(mint, v)
			if err != nil {
				return mintQuote, proofsState, fmt.Errorf("m.CheckMintRequest(pool, mint,v ) %w", err)
			}
			mintQuote = append(mintQuote, quote)
		}
	case cashu.ProofStateWs:
		proofsState, err := m.CheckProofState(mint, filters)
		if err != nil {
			return mintQuote, proofsState, fmt.Errorf("m.CheckMintRequest(pool, mint,v ) %w", err)
		}

	}

	return mintQuote, proofsState, nil
}
