package routes

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func v1WebSocketRoute(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	v1.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		var request cashu.WsRequest

		err = conn.ReadJSON(&request)
		if err != nil {
			return
		}

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

		statusChecker := m.GetCorrectStatusChecker(request)

		err = statusChecker.WatchForChanges(mint, conn)
		if err != nil {
			logger.Error("statusChecker.WatchForChanges(pool, mint, conn)", slog.String(utils.LogExtraInfo, err.Error()))
			return
		}

	})

}
func CheckStatusesOfSubscription(subKind cashu.SubscriptionKind, filters []string, pool *pgxpool.Pool, mint *m.Mint) ([]cashu.PostMintQuoteBolt11Response, []cashu.CheckState, error) {
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

func registerSubscription(request cashu.WsRequest) {

	switch request.Method {
	case cashu.Subcribe:
	case cashu.Unsubcribe:

	}

}
