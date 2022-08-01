package pubsub

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

type PubSub struct {
	melody   *melody.Melody
	clients  ClientStore
	channels ChannelStore
}

func New() *PubSub {
	instance := PubSub{}
	instance.init()
	return &instance
}

type handlerFunc func(c *Client)
type handlerDataFunc func(c *Client, data []byte)

func (p *PubSub) RegisterPubSubChannel(channelName string, handlers MessageHandlers) {
	p.channels.Register(channelName, handlers)
}

func (p *PubSub) HandleRequest(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error {
	return p.melody.HandleRequestWithKeys(writer, request, properties)
}

func (p *PubSub) IsSubscribed(channelPath string, clientId string) bool {
	if found, channel, _ := p.channels.Get(channelPath); found {
		return channel.IsSubscribed(channelPath, clientId)
	}
	return false
}

func (p *PubSub) mapEventToClient(handler handlerFunc) func(*melody.Session) {
	return func(s *melody.Session) {
		id, _ := s.Get("id")
		client := p.clients.Get(id.(string))
		handler(client)
	}
}

func (p *PubSub) mapDataEventToClient(handler handlerDataFunc) func(*melody.Session, []byte) {
	return func(s *melody.Session, data []byte) {
		id, _ := s.Get("id")
		client := p.clients.Get(id.(string))
		handler(client, data)
	}
}

func (p *PubSub) init() {
	p.melody = melody.New()
	p.clients.init()
	p.channels.init()
	p.melody.HandleConnect(p.connectHandler)
	p.melody.HandleDisconnect(p.mapEventToClient(p.disconnectHandler))
	p.melody.HandleMessage(p.mapDataEventToClient(p.messageHandler))
}

func (p *PubSub) connectHandler(s *melody.Session) {
	client := p.clients.Join(s, s.Keys())
	s.Set("id", client.Id)
}

func (p *PubSub) disconnectHandler(c *Client) {
	p.channels.UnsubscribeAll(c.Id)
	p.clients.Remove(c.Id)
}

func (p *PubSub) messageHandler(c *Client, msg []byte) {
	var req Message
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal request")
		return
	}

	switch req.Type {
	case MessageTypeSubscribe:
		p.channels.Subscribe(c, req.Channel)
	case MessageTypeUnsubscribe:
		p.channels.Unsubscribe(c.Id, req.Channel)
	case MessageTypeChannelMessage:
		p.channels.OnMessage(c, &req)
	default:
		log.WithField("type", req.Type).Warn("unknown pubsub websocket request type")
	}
}
