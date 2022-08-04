package realtime

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"strings"
)

const channelPathSep = "/"

type SubscriptionMiddleware func(s *Context) *Error
type EventHandlerFunc func(s *Context)
type MessageHandlerFunc func(s *Context, message *Message)

type ChannelHandlers struct {
	OnSubscribe             EventHandlerFunc
	OnUnsubscribe           EventHandlerFunc
	OnMessage               MessageHandlerFunc
	SubscriptionMiddlewares []SubscriptionMiddleware
}

type Channel struct {
	path        []string
	handlers    ChannelHandlers
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
			continue
		}
		return false, nil
	}
	return true, params
}

func (c *Channel) Subscribe(context *Context) {
	for _, middleware := range c.handlers.SubscriptionMiddlewares {
		if err := middleware(context); err != nil {
			sentry.CaptureException(errors.New(err.Description))
			if err := context.SendError(err); err != nil {
				sentry.CaptureException(err)
			}
			return
		}
	}

	c.subscribers.Add(context)

	if c.handlers.OnSubscribe != nil {
		c.handlers.OnSubscribe(context)
	}
}

func (c *Channel) HandleMessage(client *Client, message *Message) {
	if c.handlers.OnMessage == nil {
		return
	}

	if context, ok := c.subscribers.GetContext(message.Channel, client.Id); ok {
		c.handlers.OnMessage(context, message)
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
	if c.handlers.OnUnsubscribe != nil {
		c.handlers.OnUnsubscribe(context)
	}

	return true
}

func (c *Channel) UnsubscribeAllPaths(clientId string) bool {
	removed := c.subscribers.RemoveAllPaths(clientId)

	if c.handlers.OnUnsubscribe != nil {
		for _, context := range removed {
			c.handlers.OnUnsubscribe(context)
		}
	}

	return true
}
