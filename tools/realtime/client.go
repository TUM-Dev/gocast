package realtime

// MessageSendFunc is a function that sends a message over the network.
type MessageSendFunc func(message []byte) error

// Client is a subscriber one or multiple channels.
type Client struct {
	Id          string
	sendMessage MessageSendFunc
	properties  map[string]interface{}
}

// NewClient creates a Client
func NewClient(sendMessage MessageSendFunc, properties map[string]interface{}) *Client {
	return &Client{
		Id:          "",
		sendMessage: sendMessage,
		properties:  properties,
	}
}

// Send sends the message using the client's MessageSendFunc.
func (client *Client) Send(message []byte) error {
	return client.sendMessage(message)
}

// Get returns a property from the client's properties or (any, false)
func (client *Client) Get(key string) (value interface{}, exists bool) {
	if val, ok := client.properties[key]; ok {
		return val, ok
	}
	return nil, false
}

// Set sets a property on the client's properties.
func (client *Client) Set(key string, value interface{}) {
	client.properties[key] = value
}
