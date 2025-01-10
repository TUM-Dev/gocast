package realtime

import (
	"strings"
)

// ChannelStore stores pointers to all Channels
type ChannelStore struct {
	channels map[string]*Channel
}

func (s *ChannelStore) init() {
	s.channels = map[string]*Channel{}
}

func (s *ChannelStore) Register(path string, handlers ChannelHandlers) *Channel {
	channel := Channel{
		path:        strings.Split(path, channelPathSep),
		handlers:    handlers,
		subscribers: ChannelSubscribers{},
	}
	channel.subscribers.init()
	s.channels[path] = &channel
	return &channel
}

func (s *ChannelStore) Get(path string) (bool, *Channel, map[string]string) {
	if channel, ok := s.channels[path]; ok {
		return true, channel, map[string]string{}
	}

	for _, channel := range s.channels {
		if ok, params := channel.PathMatches(path); ok {
			return true, channel, params
		}
	}

	return false, nil, nil
}

func (s *ChannelStore) OnMessage(client *Client, message *Message) {
	if ok, channel, _ := s.Get(message.Channel); ok {
		channel.HandleMessage(client, message)
		return
	}

	logger.Warn("unknown channel on websocket message", "channel", message.Channel)
}

func (s *ChannelStore) Subscribe(client *Client, channelPath string) bool {
	if found, channel, params := s.Get(channelPath); found {
		channel.Subscribe(&Context{
			Client:     client,
			FullPath:   channelPath,
			params:     params,
			properties: map[string]interface{}{},
		})
		return true
	}
	return false
}

func (s *ChannelStore) Unsubscribe(clientId string, channelPath string) bool {
	if found, channel, _ := s.Get(channelPath); found {
		return channel.Unsubscribe(clientId, channelPath)
	}
	return false
}

func (s *ChannelStore) UnsubscribeAll(clientId string) {
	for _, channel := range s.channels {
		channel.UnsubscribeAllPaths(clientId)
	}
}
