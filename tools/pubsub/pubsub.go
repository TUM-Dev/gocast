package pubsub

import (
	"encoding/json"
	"errors"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type WSPubSubMessage struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel"`
	Payload json.RawMessage `json:"payload"`
}

type PubSub = struct {
	clients  ClientStore
	channels ChannelStore
}

func (p *PubSub) RegisterPubSubChannel(channelName string, handlers MessageHandlers) {
	p.channels.Register(channelName, handlers)
}

func (p *PubSub) Init(melody *melody.Melody) {
	melody.HandleConnect(p.connectHandler)
	melody.HandleDisconnect(p.disconnectHandler)
	melody.HandleMessage(p.MessageHandler)
}

func (p *PubSub) connectHandler(s *melody.Session) {
	id, _ := s.Get("id")   // get client id
	ctx, _ := s.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	p.clients
}

func (p *PubSub) disconnectHandler(s *melody.Session) {
	id, _ := s.Get("id")

	pubSubClientsMutex.Lock()
	delete(pubSubClients, id.(string))
	for _, channel := range pubSubChannels {
		channel.mutex.Lock()
		if context, ok := channel.subscribers[id.(string)]; ok {
			delete(channel.subscribers, id.(string))
			if channel.handlers.onUnsubscribe != nil {
				channel.handlers.onUnsubscribe(context)
			}
		}
		channel.mutex.Unlock()
	}
	pubSubClientsMutex.Unlock()
}

func messageHandler(s *melody.Session, msg []byte) {
	ctx, _ := s.Get("ctx") // get gin context
	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		ctx.(*gin.Context).AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		return
	}

	var req WSPubSubMessage
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal request")
		return
	}

	switch req.Type {
	case PubSubMessageTypeSubscribe:
		subscribeClientToChannel(s, req.Channel)
	case PubSubMessageTypeUnsubscribe:
		unsubscribeFromChannel(s, req.Channel)
	case PubSubMessageTypeChannelMessage:
		handleChannelMessage(s, &req)
	default:
		log.WithField("type", req.Type).Warn("unknown pubsub websocket request type")
	}
}

func BroadcastToPubSubChannel(channelName string, payload gin.H) error {
	channel, ok := pubSubChannels[channelName]
	if !ok {
		return errors.New("ChannelName does not exist")
	}

	message, _ := json.Marshal(gin.H{"channel": channelName, "payload": payload})

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	for clientId, _ := range channel.subscribers {
		pubSubClientsMutex.Lock()
		if subscriberSession, ok := pubSubClients[clientId]; ok {
			if err := subscriberSession.session.Write(message); err != nil {
				log.WithError(err).Warn("failed to send broadcast message to subscriber")
			}
		}
		pubSubClientsMutex.Unlock()
	}

	return nil
}

func SendInPubSubChannel(channelName string, clientId string, payload gin.H) error {
	channel, ok := pubSubChannels[channelName]
	if ok {
		return errors.New("ChannelName does not exist")
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	if _, ok := channel.subscribers[clientId]; ok {
		return errors.New("no subscriber found")
	}

	pubSubClientsMutex.Lock()
	subscriberSession, ok := pubSubClients[clientId]
	if !ok {
		return errors.New("subscriber session not found")
	}
	pubSubClientsMutex.Unlock()

	message, _ := json.Marshal(gin.H{"channel": channelName, "payload": payload})
	return subscriberSession.session.Write(message)
}

func subscribeClientToChannel(s *melody.Session, channelName string) {
	clientId, _ := s.Get("id")

	// TODO: Somehow match the channel name also with Params! :)
	channel, ok := pubSubChannels[channelName]
	if !ok {
		log.Warn("client tried to subscribe to non existing channel")
	}

	pubSubClientsMutex.Lock()
	defer pubSubClientsMutex.Unlock()

	var context = PubSubContext{
		Client: pubSubClients[clientId.(string)],
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	channel.subscribers[clientId.(string)] = &context

	if channel.handlers.onSubscribe != nil {
		channel.handlers.onSubscribe(&context)
	}
}

func unsubscribeFromChannel(s *melody.Session, channelName string) {
	clientId, _ := s.Get("id")
	channel, ok := pubSubChannels[channelName]
	if !ok {
		log.Warn("client tried to unsubscribe to non existing channel")
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	if context, ok := channel.subscribers[clientId.(string)]; ok {
		delete(channel.subscribers, clientId.(string))

		if channel.handlers.onUnsubscribe != nil {
			channel.handlers.onUnsubscribe(context)
		}
	}
}

func handleChannelMessage(s *melody.Session, req *WSPubSubMessage) {
	clientId, _ := s.Get("id")
	channel, ok := pubSubChannels[req.Channel]
	if !ok {
		log.WithField("type", req.Type).Warn("unknown channel on websocket message")
		return
	}

	if context, ok := channel.subscribers[clientId.(string)]; ok {
		if channel.handlers.onMessage != nil {
			channel.handlers.onMessage(context, req)
		}
	}
}
