package realtime

import (
	"encoding/json"
	"github.com/gabstv/melody"
	log "github.com/sirupsen/logrus"
	"net/http"
)

const (
	MessageTypeSubscribe      = "subscribe"
	MessageTypeUnsubscribe    = "unsubscribe"
	MessageTypeChannelMessage = "message"
)

type Message struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel"`
	Payload json.RawMessage `json:"payload"`
}

type Realtime struct {
	connector *Connector
	clients   ClientStore
	channels  ChannelStore
}

// New Creates a new Realtime instance
func New(connector *Connector) *Realtime {
	r := Realtime{}

	r.connector = connector
	r.clients.init()
	r.channels.init()
	r.connector.hook(&Hooks{
		OnConnect:    r.connectHandler,
		OnDisconnect: r.disconnectHandler,
		OnMessage:    r.messageHandler,
	})

	return &r
}

type handlerFunc func(c *Client)
type handlerDataFunc func(c *Client, data []byte)

// RegisterChannel registers a new channel
func (r *Realtime) RegisterChannel(channelName string, handlers ChannelHandlers) *Channel {
	return r.channels.Register(channelName, handlers)
}

// HandleRequest handles a new websocket request, adds the properties to the new client
func (r *Realtime) HandleRequest(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error {
	return r.connector.requestHandler(writer, request, properties)
}

// IsSubscribed checks whether a client is subscribed to a certain channelPath or not
func (r *Realtime) IsSubscribed(channelPath string, clientId string) bool {
	if found, channel, _ := r.channels.Get(channelPath); found {
		return channel.IsSubscribed(channelPath, clientId)
	}
	return false
}

// mapEventToClient maps a melody callback to a realtime callback with the according Client
func (r *Realtime) mapEventToClient(handler handlerFunc) func(*melody.Session) {
	return func(s *melody.Session) {
		id, _ := s.Get("id")
		client := r.clients.Get(id.(string))
		handler(client)
	}
}

// mapDataEventToClient maps a melody callback with data to a realtime callback with the according Client
func (r *Realtime) mapDataEventToClient(handler handlerDataFunc) func(*melody.Session, []byte) {
	return func(s *melody.Session, data []byte) {
		id, _ := s.Get("id")
		client := r.clients.Get(id.(string))
		handler(client, data)
	}
}

// connectHandler handles a new melody connection
func (r *Realtime) connectHandler(client *Client) {}

// disconnectHandler handles a client disconnect
func (r *Realtime) disconnectHandler(c *Client) {
	r.channels.UnsubscribeAll(c.Id)
}

// messageHandler handles a new client message
func (r *Realtime) messageHandler(c *Client, msg []byte) {
	var req Message
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal request")
		return
	}

	switch req.Type {
	case MessageTypeSubscribe:
		r.channels.Subscribe(c, req.Channel)
	case MessageTypeUnsubscribe:
		r.channels.Unsubscribe(c.Id, req.Channel)
	case MessageTypeChannelMessage:
		r.channels.OnMessage(c, &req)
	default:
		log.WithField("type", req.Type).Warn("unknown pubsub websocket request type")
	}
}
