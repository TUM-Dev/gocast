package realtime

import (
	"encoding/json"
)

type Context struct {
	Client     *Client
	FullPath   string
	params     map[string]string
	properties map[string]interface{}
}

type Error struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

func NewError(code int, description string) *Error {
	return &Error{
		Code:        code,
		Description: description,
	}
}

func (context *Context) Get(key string) (value interface{}, exists bool) {
	if val, ok := context.properties[key]; ok {
		return val, ok
	}
	return nil, false
}

func (context *Context) Set(key string, value interface{}) {
	context.properties[key] = value
}

func (context *Context) SendError(error *Error) error {
	data, err := json.Marshal(error)
	if err != nil {
		return err
	}
	message := Message{
		Type:    MessageTypeChannelMessage,
		Channel: context.FullPath,
		Payload: data,
	}
	data, err = json.Marshal(message)
	if err != nil {
		return err
	}
	return context.Client.Session.Write(data)
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
