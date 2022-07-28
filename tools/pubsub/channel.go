package pubsub

import (
	"strings"
)

const channelPathSep = "/"

type EventHandlerFunc func(s *Context)
type MessageHandlerFunc func(s *Context, message *WSPubSubMessage)

type MessageHandlers = struct {
	onSubscribe   EventHandlerFunc
	onUnsubscribe EventHandlerFunc
	onMessage     MessageHandlerFunc
}

type Channel = struct {
	path        []string
	handlers    MessageHandlers
	subscribers ChannelSubscribers
}

func (c *Channel) PathMatches(path string) (bool, map[string]string) {
	params := map[string]string{}
	parts := strings.Split(path, channelPathSep)

	if len(c.path) != len(parts) {
		return false, nil
	}

	for i, s := range parts {
		if c.path[i] == s {
			continue
		}
		if c.path[i][0] == ':' {
			params[c.path[i][1:]] = s
		}
		return false, nil
	}
	return true, params
}

func (c *Channel) Subscribe(context *Context) {
	c.subscribers.Add(context)

	if c.handlers.onSubscribe != nil {
		c.handlers.onSubscribe(context)
	}
}

func (c *Channel) Unsubscribe(context *Context) {
	c.subscribers.Remove(context.ClientId)

	if c.handlers.onUnsubscribe != nil {
		c.handlers.onUnsubscribe(context)
	}
}
