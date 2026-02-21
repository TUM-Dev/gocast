package realtime

import (
	"encoding/json"
)

// Context carries params and properties across a clients lifetime.
type Context struct {
	Client     *Client
	FullPath   string
	params     map[string]string
	properties map[string]interface{}
}

// Error represents an error response.
type Error struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

// NewError creates the error based on code and description.
func NewError(code int, description string) *Error {
	return &Error{
		Code:        code,
		Description: description,
	}
}

// Get returns a key from the Context
func (context *Context) Get(key string) (value interface{}, exists bool) {
	if val, ok := context.properties[key]; ok {
		return val, ok
	}
	return nil, false
}

// Set sets a key on the Context
func (context *Context) Set(key string, value interface{}) {
	context.properties[key] = value
}

// SendError flushes an error to the Context's client.
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
	return context.Client.Send(data)
}

// Send sends a generic payload to the Context's client.
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
	return context.Client.Send(data)
}

// SetParams sets params on Context
func (context *Context) SetParams(params map[string]string) {
	context.params = params
}

// Param returns a param by key.
func (context *Context) Param(key string) string {
	return context.params[key]
}
