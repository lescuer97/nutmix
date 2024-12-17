package mint

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

func SendJson(conn *websocket.Conn, content any) error {
	contentToSend, err := json.Marshal(content)
	if err != nil {
		return err
	}

	err = conn.WriteMessage(websocket.TextMessage, contentToSend)
	if err != nil {
		return err
	}

	return nil
}
