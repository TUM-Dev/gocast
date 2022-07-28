package pubsub

type Context struct {
	Client   *Client
	FullPath string
	params   map[string]string
}

func (context *Context) SetParams(params map[string]string) {
	context.params = params
}

func (context *Context) Param(key string) string {
	return context.params[key]
}
