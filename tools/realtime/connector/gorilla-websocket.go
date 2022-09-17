package connector

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joschahenningsen/TUM-Live/tools/realtime"
	"net/http"
)

func NewGorillaWebsocketConnector() *realtime.Connector {
	var upgrader = websocket.Upgrader{} // use default options
	var connector *realtime.Connector
	connector = realtime.NewConnector(
		func(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error {
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				return err
			}

			client := connector.Join(
				func(message []byte) error {
					return conn.WriteMessage(websocket.TextMessage, message)
				},
				properties,
			)

			go func() {
				defer (func() {
					err := conn.Close()
					if err != nil {
						fmt.Println("Error during closing connection:", err)
					}
					connector.Leave(client.Id)
				})()

				for {
					_, message, err := conn.ReadMessage()
					if err != nil {
						fmt.Println("Error during message reading:", err)
						break
					}
					connector.Message(client.Id, message)
				}
			}()

			return nil
		},
	)

	return connector
}
