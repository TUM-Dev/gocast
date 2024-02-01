package realtime

import (
	"errors"
	"strings"

	"github.com/getsentry/sentry-go"
)

// channelPathSep describes the separator of paths in a channel name. e.g 'stream/123' is seperated by channelPathSep
const channelPathSep = "/"

// SubscriptionMiddleware is a function that is executed when a client connects to a Channel.
// If the middleware returns a non nil Error, the subscription won't be finished.
type SubscriptionMiddleware func(s *Context) *Error

// EventHandlerFunc is a function that is executed when subscribing or unsubscribing to the Channel.
type EventHandlerFunc func(s *Context)

// MessageHandlerFunc is a function that executes when a message is sent to the Channel.
type MessageHandlerFunc func(s *Context, message *Message)

// ChannelHandlers contains all handler functions for various events in the Channel.
type ChannelHandlers struct {
	OnSubscribe             EventHandlerFunc
	OnUnsubscribe           EventHandlerFunc
	OnMessage               MessageHandlerFunc
	SubscriptionMiddlewares []SubscriptionMiddleware
}

// Channel describes a room, websocket users can subscribe and sent messages to.
type Channel struct {
	path        []string
	handlers    ChannelHandlers
	subscribers ChannelSubscribers
}

// PathMatches returns true and the params of the channel subscription if the path matches the path of the Channel.
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

// Subscribe executes the Channels middlewares and(if successful) adds the user to the Channel and executes the channels OnSubscribe handler.
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

// HandleMessage executes the channels OnMessage method if it exists.
func (c *Channel) HandleMessage(client *Client, message *Message) {
	if c.handlers.OnMessage == nil {
		return
	}

	if context, ok := c.subscribers.GetContext(client.Id, message.Channel); ok {
		c.handlers.OnMessage(context, message)
	}
}

// IsSubscribed returns true if the client is connected to the channel
func (c *Channel) IsSubscribed(clientId string, path string) bool {
	return c.subscribers.IsSubscribed(clientId, path)
}

func (c *Channel) FindContext(clientId string, path string) (*Context, bool) {
	return c.subscribers.GetContext(clientId, path)
}

// Unsubscribe removes the client from the channel and executes the OnUnsubscribe handler
func (c *Channel) Unsubscribe(clientId string, path string) bool {
	context, isSubscriber := c.subscribers.GetContext(clientId, path)
	if !isSubscriber {
		return false
	}

	c.subscribers.Remove(clientId, path)
	if c.handlers.OnUnsubscribe != nil {
		c.handlers.OnUnsubscribe(context)
	}

	return true
}

// UnsubscribeAllPaths unsubscribes a client from all paths of the channel they are connected to.
func (c *Channel) UnsubscribeAllPaths(clientId string) bool {
	removed := c.subscribers.RemoveAllPaths(clientId)

	if c.handlers.OnUnsubscribe != nil {
		for _, context := range removed {
			c.handlers.OnUnsubscribe(context)
		}
	}

	return true
}
