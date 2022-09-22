package realtime

import "github.com/gabstv/melody"

type Client struct {
	Id         string
	Session    *melody.Session
	properties map[string]interface{}
}

func (client *Client) Get(key string) (value interface{}, exists bool) {
	if val, ok := client.properties[key]; ok {
		return val, ok
	}
	return nil, false
}

func (client *Client) Set(key string, value interface{}) {
	client.properties[key] = value
}
