package realtime

import (
	"net/http"
)

type (
	// ConnectHookFunc is called on connecting.
	ConnectHookFunc func(*Client)
	// DisconnectHookFunc is called on disconnecting.
	DisconnectHookFunc func(*Client)
	// MessageHookFunc is called on Message.
	MessageHookFunc func(*Client, []byte)
	// RequestHandlerFunc is a func that handles a request.
	RequestHandlerFunc func(writer http.ResponseWriter, request *http.Request, properties map[string]interface{}) error
)

// Connector manages a set of clients
type Connector struct {
	requestHandler RequestHandlerFunc
	clients        ClientStore
	hooks          *Hooks
}

// Hooks manages the Connector's Hooks
type Hooks struct {
	OnConnect    ConnectHookFunc
	OnDisconnect DisconnectHookFunc
	OnMessage    MessageHookFunc
}

// NewConnector creates a connector for the given RequestHandlerFunc
func NewConnector(requestHandler RequestHandlerFunc) *Connector {
	connector := &Connector{
		requestHandler: requestHandler,
		hooks:          &Hooks{},
	}
	connector.clients.init()
	return connector
}

// Join To be triggered if a client connects via ws
func (c *Connector) Join(sendMessage MessageSendFunc, properties map[string]interface{}) *Client {
	client := NewClient(sendMessage, properties)
	c.clients.Join(client)
	if c.hooks.OnConnect != nil {
		c.hooks.OnConnect(client)
	}
	return client
}

// Message triggers the clients OnMessage handler
func (c *Connector) Message(clientId string, data []byte) {
	client := c.clients.Get(clientId)
	if c.hooks.OnMessage != nil {
		c.hooks.OnMessage(client, data)
	}
}

// Leave triggers the clients OnDisconnect handler and removes it from the Connector
func (c *Connector) Leave(clientId string) {
	client := c.clients.Get(clientId)
	if client == nil {
		return
	}
	// Removal must happen whatever a handler does: a panic escaping OnDisconnect used to
	// skip it, retaining the client and everything its context points at forever.
	defer c.clients.Remove(client.Id)
	if c.hooks.OnDisconnect != nil {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic in OnDisconnect hook", "client", client.Id, "err", r)
			}
		}()
		c.hooks.OnDisconnect(client)
	}
}

func (c *Connector) hook(hooks *Hooks) {
	c.hooks = hooks
}
