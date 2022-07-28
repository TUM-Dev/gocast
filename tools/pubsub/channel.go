package pubsub

import (
	"strings"
)

const channelPathSep = "/"

type EventHandlerFunc func(s *Context)
type MessageHandlerFunc func(s *Context, message *Message)

type MessageHandlers struct {
	onSubscribe   EventHandlerFunc
	onUnsubscribe EventHandlerFunc
	onMessage     MessageHandlerFunc
}

type Channel struct {
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

func (c *Channel) HandleMessage(client *Client, message *Message) {
	if c.handlers.onMessage == nil {
		return
	}

	if context, ok := c.subscribers.GetContext(message.Channel, client.Id); ok {
		c.handlers.onMessage(context, message)
	}
}

func (c *Channel) IsSubscribed(path string, clientId string) bool {
	return c.subscribers.IsSubscribed(path, clientId)
}

func (c *Channel) Unsubscribe(path string, clientId string) bool {
	context, isSubscriber := c.subscribers.GetContext(path, clientId)
	if !isSubscriber {
		return false
	}

	c.subscribers.Remove(path, clientId)
	if c.handlers.onUnsubscribe != nil {
		c.handlers.onUnsubscribe(context)
	}

	return true
}

func (c *Channel) UnsubscribeAllPaths(clientId string) bool {
	removed := c.subscribers.RemoveAllPaths(clientId)

	if c.handlers.onUnsubscribe != nil {
		for _, context := range removed {
			c.handlers.onUnsubscribe(context)
		}
	}

	return true
}
