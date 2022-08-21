package realtime

import (
	"github.com/gabstv/melody"
	"net/http"
)

type ConnectHookFunc func(*Client)
type DisconnectHookFunc func(*Client)
type MessageHookFunc func(*Client, []byte)
type RequestHandlerFunc func(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error

type Connector struct {
	requestHandler RequestHandlerFunc
	clients        ClientStore
	hooks          *Hooks
}

type Hooks struct {
	OnConnect    ConnectHookFunc
	OnDisconnect DisconnectHookFunc
	OnMessage    MessageHookFunc
}

func (c *Connector) hook(hooks *Hooks) {
	c.hooks = hooks
}

func NewMelodyConnector() *Connector {
	melodyInstance := melody.New()
	connector := &Connector{
		hooks: &Hooks{},
		requestHandler: func(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error {
			return melodyInstance.HandleRequestWithKeys(writer, request, properties)
		},
	}
	connector.clients.init()

	melodyInstance.HandleConnect(func(s *melody.Session) {
		client := Client{
			Id: "",
			sendMessage: func(message []byte) error {
				return s.Write(message)
			},
			properties: s.Keys(),
		}

		connector.clients.Join(&client)
		s.Set("id", client.Id)

		if connector.hooks.OnConnect != nil {
			connector.hooks.OnConnect(&client)
		}
	})

	melodyInstance.HandleDisconnect(func(s *melody.Session) {
		id, _ := s.Get("id")
		client := connector.clients.Get(id.(string))
		if connector.hooks.OnDisconnect != nil {
			connector.hooks.OnDisconnect(client)
		}
		connector.clients.Remove(client.Id)
	})

	melodyInstance.HandleMessage(func(s *melody.Session, data []byte) {
		id, _ := s.Get("id")
		client := connector.clients.Get(id.(string))
		if connector.hooks.OnMessage != nil {
			connector.hooks.OnMessage(client, data)
		}
	})

	return connector
}
