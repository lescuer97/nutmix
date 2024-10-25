package routes

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
)

func v1WebSocketRoute(r *gin.Engine, pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) {
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

		for {
            var request cashu.WsRequest

			err := conn.ReadJSON(&request)
			if err != nil {
				return
			}

            fmt.Printf("receive message: %+v", request)

            // err := conn.WriteJSON()

			conn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!"))
			time.Sleep(time.Second)
		}
	})

}
