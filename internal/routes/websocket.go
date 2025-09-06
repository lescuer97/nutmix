package routes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/mint"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lightningnetwork/lnd/zpay32"
)

var ErrAlreadySubscribed = errors.New("Filter already subscribed")

func checkOrigin(r *http.Request) bool {
	return true
}

func v1WebSocketRoute(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     checkOrigin,
	}

	v1.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Warn("upgrader.Upgrade(c.Writer, c.Request, nil)", slog.Any("error", err))
			return
		}
		defer conn.Close()

		var request cashu.WsRequest

		err = conn.ReadJSON(&request)
		if err != nil {
			return
		}

		proofChan := make(chan cashu.Proof, 2)
		mintChan := make(chan cashu.MintRequestDB, 1)
		meltChan := make(chan cashu.MeltRequestDB, 1)
		closeChan := make(chan string, 1)
		// parse request check if subscription or unsubscribe
		err = handleWSRequest(request, mint.Observer, proofChan, mintChan, meltChan, closeChan)

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
			slog.Error("Error on creating websocket", slog.Any("error", err))
			return
		}

		slog.Debug("New request", slog.Any("request", request))
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
			slog.Warn("m.SendJson(conn, response)", slog.Any("error", err))
			return
		}
		err = CheckStatusOfSub(request, mint, conn)
		if err != nil {
			slog.Warn("CheckStatusOfSub(request, mint,conn)", slog.Any("error", err))
			return
		}

		listenError := make(chan error)
		go ListenToIncommingMessage(mint.Observer, conn, listenError, proofChan, mintChan, meltChan, closeChan)

		for {
			select {
			case error := <-listenError:
				slog.Warn("go ListenToIncommingMessage(&activeSubs, conn, listining).", slog.Any("error", error))
				return

			case proof, ok := <-proofChan:
				if ok {
					statusNotif := cashu.WsNotification{
						JsonRpc: "2.0",
						Method:  cashu.Subcribe,
						Params: cashu.WebRequestParams{
							SubId: request.Params.SubId,
						},
					}
					state := cashu.CheckState{Y: proof.Y, State: proof.State, Witness: &proof.Witness}
					statusNotif.Params.Payload = state

					err = m.SendJson(conn, statusNotif)
					if err != nil {
						slog.Warn("m.SendJson(conn, response)", slog.Any("error", err))
						return
					}
				}

			case mintState, ok := <-mintChan:
				if ok {
					statusNotif := cashu.WsNotification{
						JsonRpc: "2.0",
						Method:  cashu.Subcribe,
						Params: cashu.WebRequestParams{
							SubId: request.Params.SubId,
						},
					}
					statusNotif.Params.Payload = mintState.PostMintQuoteBolt11Response()

					err = m.SendJson(conn, statusNotif)
					if err != nil {
						slog.Warn("m.SendJson(conn, response)", slog.Any("error", err))
						return
					}
				}
			case meltState, ok := <-meltChan:
				if ok {
					statusNotif := cashu.WsNotification{
						JsonRpc: "2.0",
						Method:  cashu.Subcribe,
						Params: cashu.WebRequestParams{
							SubId: request.Params.SubId,
						},
					}
					statusNotif.Params.Payload = meltState.GetPostMeltQuoteResponse()

					err = m.SendJson(conn, statusNotif)
					if err != nil {
						slog.Warn("m.SendJson(conn, response)", slog.Any("error", err))
						return
					}
				}

			case _ = <-closeChan:
				return
			}
		}

	})
}

func handleWSRequest(request cashu.WsRequest, observer *mint.Observer, proofChan chan cashu.Proof, mintChan chan cashu.MintRequestDB, meltChan chan cashu.MeltRequestDB,
	closeChan chan string,
) error {
	switch request.Method {
	case cashu.Subcribe:

		switch request.Params.Kind {
		case cashu.ProofStateWs:
			for _, filter := range request.Params.Filters {
				observer.AddProofWatch(filter, mint.ProofWatchChannel{Channel: proofChan, SubId: request.Params.SubId})
			}
		case cashu.Bolt11MintQuote:
			for _, filter := range request.Params.Filters {
				observer.AddMintWatch(filter, mint.MintQuoteChannel{Channel: mintChan, SubId: request.Params.SubId})
			}
		case cashu.Bolt11MeltQuote:
			for _, filter := range request.Params.Filters {
				observer.AddMeltWatch(filter, mint.MeltQuoteChannel{Channel: meltChan, SubId: request.Params.SubId})
			}

		}

	case cashu.Unsubcribe:
		go observer.RemoveWatch(request.Params.SubId)
		closeChan <- "asked for unsubscribe"
	}
	return nil
}

func ListenToIncommingMessage(subs *mint.Observer, conn *websocket.Conn, listenChannel chan error, proofChan chan cashu.Proof, mintChan chan cashu.MintRequestDB, meltChan chan cashu.MeltRequestDB, closeChan chan string) {
	for {
		var request cashu.WsRequest
		err := conn.ReadJSON(&request)
		if err != nil {
			listenChannel <- fmt.Errorf("conn.ReadJSON(&request).%w", err)
			return
		}

		err = handleWSRequest(request, subs, proofChan, mintChan, meltChan, closeChan)
		if err != nil {
			listenChannel <- fmt.Errorf("handleWSRequest(request, subs) %w", err)
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
			listenChannel <- fmt.Errorf("m.SendJson(conn, response) %w", err)
			return
		}
	}
}

func CheckStatusOfSub(request cashu.WsRequest, mint *m.Mint, conn *websocket.Conn) error {

	statusNotif := cashu.WsNotification{
		JsonRpc: "2.0",
		Method:  cashu.Subcribe,
		Params: cashu.WebRequestParams{
			SubId: request.Params.SubId,
		},
	}
	alreadyCheckedFilter := make(map[string]any)
	for _, filter := range request.Params.Filters {
		// check if a new stored notif has already been seen and if no send a status update and store state
		value, exists := alreadyCheckedFilter[filter]

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			return fmt.Errorf("m.MintDB.GetTx(ctx). %w", err)
		}
		defer mint.MintDB.Rollback(ctx, tx)

		switch request.Params.Kind {
		case cashu.Bolt11MintQuote:
			quote, err := mint.MintDB.GetMintRequestById(tx, filter)

			if err != nil {
				return fmt.Errorf("mint.MintDB.GetMintRequestById(filter). %w", err)
			}

			decodedInvoice, err := zpay32.Decode(quote.Request, mint.LightningBackend.GetNetwork())
			if err != nil {
				return fmt.Errorf("m.CheckMintRequest(mint, filter). %w", err)
			}
			mintState, err := m.CheckMintRequest(mint, quote, decodedInvoice)
			if err != nil {
				return fmt.Errorf("m.CheckMintRequest(mint, filter). %w", err)
			}
			statusNotif.Params.Payload = mintState
			if exists {
				if value.(cashu.MintRequestDB).State != mintState.State {
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

				if value.(cashu.MeltRequestDB).State != meltState.State {
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
		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			return fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err)
		}

	}
	return nil
}
