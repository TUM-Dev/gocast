package connector

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/joschahenningsen/TUM-Live/tools/realtime"
	"net/http"
)

package connector

import (
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
			defer (func () {
				conn.Close()
				connector.Leave(conn)
			})()

			// The event loop
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					log.Println("Error during message reading:", err)
					break
				}
				log.Printf("Received: %s", message)
				err = conn.WriteMessage(messageType, message)
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
			}
		},
	)

	melodyInstance.HandleConnect(func(s *melody.Session) {
		client := connector.Join(
			func(message []byte) error {
				return s.Write(message)
			},
			s.Keys(),
		)
		s.Set("id", client.Id)
	})

	melodyInstance.HandleDisconnect(func(s *melody.Session) {
		id, _ := s.Get("id")

	})

	melodyInstance.HandleMessage(func(s *melody.Session, data []byte) {
		id, _ := s.Get("id")
		connector.Message(id.(string), data)
	})

	return connector
}
