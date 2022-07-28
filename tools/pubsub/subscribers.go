package pubsub

import "sync"

type ChannelSubscribers struct {
	subscribers map[string]*Context
	mutex       sync.RWMutex
}

func createKey(path string, clientId string) string {
	return clientId + "__" + path
}

func (subs *ChannelSubscribers) IsSubscribed(path string, clientId string) bool {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	var _, exists = subs.subscribers[createKey(path, clientId)]
	return exists
}

func (subs *ChannelSubscribers) GetContext(path string, clientId string) (*Context, bool) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	var context, exists = subs.subscribers[createKey(path, clientId)]
	return context, exists
}

func (subs *ChannelSubscribers) Add(context *Context) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	subs.subscribers[createKey(context.FullPath, context.Client.Id)] = context
}

func (subs *ChannelSubscribers) RemoveAllPaths(clientId string) []*Context {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	var removed []*Context
	for key, context := range subs.subscribers {
		if context.Client.Id == clientId {
			delete(subs.subscribers, key)
			removed = append(removed, context)
		}
	}
	return removed
}

func (subs *ChannelSubscribers) Remove(path string, clientId string) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	delete(subs.subscribers, createKey(path, clientId))
}
