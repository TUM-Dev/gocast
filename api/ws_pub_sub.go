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
	"sync"
)

const (
	PubSubMessageTypeSubscribe      = "subscribe"
	PubSubMessageTypeUnsubscribe    = "unsubscribe"
	PubSubMessageTypeChannelMessage = "message"
)

var pubSubMelody *melody.Melody

var pubSubClientsMutex sync.RWMutex
var pubSubClients = map[string]*WSPubSubClient{}
var pubSubChannels = map[string]*PubSubChannel{}

type PubSubHandlerFunc func(*melody.Session)

type WSPubSubClient = struct {
	session *melody.Session
	user    *model.User
}

type PubSubChannel = struct {
	channelName string
	handler     PubSubHandlerFunc
	subscribers map[string]bool
	mutex       sync.RWMutex
}

type wsPubSubMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
}

// RegisterPubSubChannel registers a pubSub channel handler for a specific channelName
func RegisterPubSubChannel(channelName string, handler PubSubHandlerFunc) {
	pubSubChannels[channelName] = &PubSubChannel{
		channelName: channelName,
		handler:     handler,
		subscribers: map[string]bool{},
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
	routes := liveUpdateRoutes{daoWrapper}

	if pubSubMelody == nil {
		log.Printf("creating melody")
		liveMelody = melody.New()
	}
	pubSubMelody.HandleConnect(wsPubSubConnectionHandler)
	pubSubMelody.HandleDisconnect(wsPubSubDisconnectHandler)
	m.HandleMessage(wsPubSubMessageHandler)

	router.GET("/ws", routes.handleWSPubSubConnect)
}

func (r liveUpdateRoutes) handleWSPubSubConnect(c *gin.Context) {
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
		delete(channel.subscribers, id.(string))
		channel.mutex.Unlock()
	}
	pubSubClientsMutex.Unlock()
}

func subscribeClientToChannel(clientId string, channelName string) {
	channel, ok := pubSubChannels[channelName]
	if !ok {
		log.Warn("client tried to subscribe to non existing channel")
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	channel.subscribers[clientId] = true
}

func unsubscribeFromChannel(clientId string, channelName string) {
	channel, ok := pubSubChannels[channelName]
	if !ok {
		log.Warn("client tried to unsubscribe to non existing channel")
	}

	channel.mutex.Lock()
	defer channel.mutex.Unlock()

	delete(channel.subscribers, clientId)
}

func wsPubSubMessageHandler(s *melody.Session, msg []byte) {
	id, _ := s.Get("id")   // get gin context
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

	var req wsPubSubMessage
	err := json.Unmarshal(msg, &req)
	if err != nil {
		log.WithError(err).Warn("could not unmarshal request")
		return
	}

	switch req.Type {
	case PubSubMessageTypeSubscribe:
		subscribeClientToChannel(id.(string), req.Channel)
	case PubSubMessageTypeUnsubscribe:
		unsubscribeFromChannel(id.(string), req.Channel)
	case PubSubMessageTypeChannelMessage:
		// TODO: pass data messages to right handler
	default:
		log.WithField("type", req.Type).Warn("unknown pubsub websocket request type")
	}
}
