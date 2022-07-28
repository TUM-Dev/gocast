package pubsub

type Context struct {
	params map[string]string
	Client *Client
}

func (context *Context) SetParams(params map[string]string) {
	context.params = params
}

func (context *Context) Param(key string) string {
	return context.params[key]
}
