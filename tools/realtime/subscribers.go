package realtime

import "sync"

type ChannelSubscribers struct {
	subscribers map[string]*Context
	mutex       sync.RWMutex
}

func createKey(clientId string, path string) string {
	return clientId + "__" + path
}

func (subs *ChannelSubscribers) init() {
	subs.subscribers = map[string]*Context{}
}

func (subs *ChannelSubscribers) IsSubscribed(clientId string, path string) bool {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	_, exists := subs.subscribers[createKey(clientId, path)]
	return exists
}

func (subs *ChannelSubscribers) GetContext(clientId string, path string) (*Context, bool) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	context, exists := subs.subscribers[createKey(clientId, path)]
	return context, exists
}

func (subs *ChannelSubscribers) Add(context *Context) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	subs.subscribers[createKey(context.Client.Id, context.FullPath)] = context
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

func (subs *ChannelSubscribers) Remove(clientId string, path string) {
	subs.mutex.Lock()
	defer subs.mutex.Unlock()
	delete(subs.subscribers, createKey(clientId, path))
}
