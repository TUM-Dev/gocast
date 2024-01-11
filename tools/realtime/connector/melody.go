package connector

import (
	"github.com/TUM-Dev/gocast/tools/realtime"
	"github.com/gabstv/melody"
	"net/http"
)

func NewMelodyConnector() *realtime.Connector {
	melodyInstance := melody.New()
	connector := realtime.NewConnector(
		func(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error {
			return melodyInstance.HandleRequestWithKeys(writer, request, properties)
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
		connector.Leave(id.(string))
	})

	melodyInstance.HandleMessage(func(s *melody.Session, data []byte) {
		id, _ := s.Get("id")
		connector.Message(id.(string), data)
	})

	return connector
}
