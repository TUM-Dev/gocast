package api

import (
	"encoding/json"
	"errors"
	"github.com/gabstv/melody"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"sync"
)

const (
	PubSubMessageTypeSubscribe      = "subscribe"
	PubSubMessageTypeUnsubscribe    = "unsubscribe"
	PubSubMessageTypeChannelMessage = "message"
)

var paramRegex = regexp.MustCompile(`:([A-Za-z\d]+)`)

var pubSubMelody *melody.Melody

var pubSubClientsMutex sync.RWMutex
var pubSubClients = map[string]*WSPubSubClient{}
var pubSubChannels = map[string]*PubSubChannel{}

type PubSubContext struct {
	params map[string]string
	Client *WSPubSubClient
}

func (psc *PubSubContext) Get(key string) (value interface{}, exists bool) {
	if psc.Client == nil || psc.Client.session == nil {
		return nil, false
	}
	return psc.Client.session.Get(key)
}

func (psc *PubSubContext) Set(key string, value interface{}) {
	if psc.Client != nil && psc.Client.session != nil {
		psc.Client.session.Set(key, value)
	}
}

func (psc *PubSubContext) Param(key string) string {
	return psc.params[key]
}

type PubSubEventHandlerFunc func(s *PubSubContext)
type PubSubMessageHandlerFunc func(s *PubSubContext, message *WSPubSubMessage)

type PubSubMessageHandlers = struct {
	onSubscribe   PubSubEventHandlerFunc
	onUnsubscribe PubSubEventHandlerFunc
	onMessage     PubSubMessageHandlerFunc
}

type WSPubSubClient = struct {
	session *melody.Session
	user    *model.User
}

type PubSubChannel = struct {
	channelName string
	handlers    PubSubMessageHandlers
	subscribers map[string]*PubSubContext
	mutex       sync.RWMutex
}

type WSPubSubMessage struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel"`
	Payload json.RawMessage `json:"payload"`
}

type wsPubSubRoutes struct {
	dao.DaoWrapper
}

func IsSubscribed(clientId string, channelName string) bool {
	channel, ok := pubSubChannels[channelName]
	if !ok {
		return false
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()
	var _, exists = channel.subscribers[clientId]
	return exists
}

// RegisterPubSubChannel registers a pubSub channel handler for a specific channelName
func RegisterPubSubChannel(channelName string, handlers PubSubMessageHandlers) {
	pubSubChannels[channelName] = &PubSubChannel{
		channelName: channelName,
		handlers:    handlers,
		subscribers: map[string]*PubSubContext{},
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

// reserveNextId Generates a new id for a client
func nextId() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	uuidString := newUUID.String()
	if pubSubClients[uuidString] != nil {
		return nextId()
	}
	return uuidString, nil
}

func configGinWSPubSubRouter(router *gin.RouterGroup, daoWrapper dao.DaoWrapper) {
	routes := wsPubSubRoutes{daoWrapper}

	if pubSubMelody == nil {
		log.Printf("creating melody")
		pubSubMelody = melody.New()
	}
	pubSubMelody.HandleConnect(wsPubSubConnectionHandler)
	pubSubMelody.HandleDisconnect(wsPubSubDisconnectHandler)
	pubSubMelody.HandleMessage(wsPubSubMessageHandler)

	router.GET("/ws", routes.handleWSPubSubConnect)
}

func (r wsPubSubRoutes) handleWSPubSubConnect(c *gin.Context) {
	id, err := nextId()
	if err != nil {
		log.WithError(err).Warn("could not generate a uuid for a ws client")
		return
	}

	ctxMap := make(map[string]interface{}, 1)
	ctxMap["id"] = id
	ctxMap["ctx"] = c
	ctxMap["dao"] = r.DaoWrapper

	_ = pubSubMelody.HandleRequestWithKeys(c.Writer, c.Request, ctxMap)
}

var wsPubSubConnectionHandler = func(s *melody.Session) {
	id, _ := s.Get("id")   // get client id
	ctx, _ := s.Get("ctx") // get gin context

	foundContext, exists := ctx.(*gin.Context).Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	pubSubClientsMutex.Lock()
	pubSubClients[id.(string)] = &WSPubSubClient{s, tumLiveContext.User}
	pubSubClientsMutex.Unlock()
}

func wsPubSubDisconnectHandler(s *melody.Session) {
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

func wsPubSubMessageHandler(s *melody.Session, msg []byte) {
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
