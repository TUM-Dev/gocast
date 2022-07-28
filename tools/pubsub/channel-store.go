package pubsub

import "strings"

type ChannelStore = struct {
	channels map[string]*Channel
}

func (c *ChannelStore) Register(path string, handlers MessageHandlers) {
	c.channels[path] = &Channel{
		path:        strings.Split(path, channelPathSep),
		handlers:    handlers,
		subscribers: ChannelSubscribers{},
	}
}

func (c *ChannelStore) Get(path string) (bool, *Channel, map[string]string) {
	if channel, ok := c.channels[path]; ok {
		return true, channel, map[string]string{}
	}

	for _, channel := range c.channels {
		if ok, params := channel.PathMatches(path); ok {
			return true, channel, params
		}
	}

	return false, nil, nil
}
