package realtime

import (
	"strings"
	"testing"
)

func TestChannelSubscribe(t *testing.T) {
	t.Run("Simple Case", func(t *testing.T) {
		simplePath := "example/path/test"
		clientId := "123789"

		store := ChannelStore{}
		store.init()
		channel := store.Register(simplePath, ChannelHandlers{})

		if result := channel.IsSubscribed(simplePath, clientId); result {
			t.Errorf("channel.IsSubscribed(%s, %s) = true, want false", simplePath, clientId)
		}

		store.Subscribe(&Client{Id: clientId}, simplePath)

		if result := channel.IsSubscribed(simplePath, clientId); result {
			t.Errorf("channel.IsSubscribed(%s, %s) = false, want true", simplePath, clientId)
		}

		store.Unsubscribe(clientId, simplePath)

		if result := channel.IsSubscribed(simplePath, clientId); result {
			t.Errorf("channel.IsSubscribed(%s, %s) = true, want false", simplePath, clientId)
		}
	})

	t.Run("With Params", func(t *testing.T) {
		var subContext *Context
		var unsubContext *Context
		path := "example/:testParam/test"
		testParam := "foo-bar"
		testPath := strings.Replace(path, ":testParam", testParam, 1)
		clientId := "123789"

		store := ChannelStore{}
		store.init()
		channel := store.Register(path, ChannelHandlers{
			OnSubscribe: func(s *Context) {
				subContext = s
			},
			OnUnsubscribe: func(s *Context) {
				unsubContext = s
			},
		})

		store.Subscribe(&Client{Id: clientId}, testPath)

		if result := channel.IsSubscribed(path, clientId); result {
			t.Errorf("channel.IsSubscribed(%s, %s) = false, want true", path, clientId)
		}

		if subContext.params["testParam"] != testParam {
			t.Errorf("subContext.params[\"testParam\"] = %s, want %s", subContext.params["testParam"], testParam)
		}

		store.Unsubscribe(clientId, testPath)

		if result := channel.IsSubscribed(testPath, clientId); result {
			t.Errorf("channel.IsSubscribed(%s, %s) = true, want false", testPath, clientId)
		}

		if unsubContext.params["testParam"] != testParam {
			t.Errorf("unsubContext.params[\"testParam\"] = %s, want %s", unsubContext.params["testParam"], testParam)
		}
	})
}
