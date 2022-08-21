package realtime

import (
	"github.com/google/uuid"
	"sync"
)

type ClientStore struct {
	clients map[string]*Client
	mutex   sync.RWMutex
}

func (c *ClientStore) init() {
	c.clients = map[string]*Client{}
}

func (c *ClientStore) NextId() string {
	var uuidGen, _ = uuid.NewUUID()
	uuidString := uuidGen.String()
	if _, ok := c.clients[uuidString]; ok {
		return c.NextId()
	}
	return uuidString
}

func (c *ClientStore) Join(client *Client) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if client.Id == "" {
		client.Id = c.NextId()
	}
	c.clients[client.Id] = client
}

func (c *ClientStore) Get(id string) *Client {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.clients[id]
}

func (c *ClientStore) Remove(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.clients, id)
}
