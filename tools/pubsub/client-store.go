package pubsub

import (
	"github.com/gabstv/melody"
	"github.com/google/uuid"
	"sync"
)

var uuidGen, _ = uuid.NewUUID()

type ClientStore = struct {
	clients map[string]*Client
	mutex   sync.RWMutex
}

func (c *ClientStore) NextId() string {
	uuidString := uuidGen.String()
	if _, ok := c.clients[uuidString]; ok {
		return c.NextId()
	}
	return uuidString
}

func (c *ClientStore) Join(session *melody.Session, props map[string]*interface{}) *Client {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	client := Client{
		Id:         c.NextId(),
		Session:    session,
		properties: props,
	}
	c.clients[client.Id] = &client
	return &client
}

func (c *ClientStore) Remove(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.clients, id)
}
