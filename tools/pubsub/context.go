package pubsub

import (
	"encoding/json"
)

type Context struct {
	Client   *Client
	FullPath string
	params   map[string]string
}

func (context *Context) Send(payload []byte) error {
	message := Message{
		Type:    MessageTypeChannelMessage,
		Channel: context.FullPath,
		Payload: payload,
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return context.Client.Session.Write(data)
}

func (context *Context) SetParams(params map[string]string) {
	context.params = params
}

func (context *Context) Param(key string) string {
	return context.params[key]
}
