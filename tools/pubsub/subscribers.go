package pubsub

import "sync"

type ChannelSubscribers = struct {
	subscribers map[string]*Context
	mutex       sync.RWMutex
}

func (subs *ChannelSubscribers) IsSubscribed(clientId string) bool {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	var _, exists = subs.subscribers[clientId]
	return exists
}

func (subs *ChannelSubscribers) Add(context *Context) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	subs.subscribers[context.Client.Id] = context
}

func (subs *ChannelSubscribers) Remove(clientId string) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	delete(subs.subscribers, clientId)
}
