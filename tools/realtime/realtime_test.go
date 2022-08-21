package realtime

import (
	"encoding/json"
	"testing"
)

type FakeSocketWriteFunc func(msg []byte) error
type FakeSocketHandleConnectFunc func(session *FakeSocketSession)
type FakeSocketHandleDisconnectFunc func(session *FakeSocketSession)
type FakeSocketHandleMessageFunc func(session *FakeSocketSession, msg []byte)

type FakeSocket struct {
	OnWrite          FakeSocketWriteFunc
	handleConnect    FakeSocketHandleConnectFunc
	handleDisconnect FakeSocketHandleDisconnectFunc
	handleMessage    FakeSocketHandleMessageFunc
}

func (f *FakeSocket) NewClientConnects() *FakeSocketSession {
	s := &FakeSocketSession{
		onDisconnect: f.handleDisconnect,
		onMessage:    f.handleMessage,
	}
	f.handleConnect(s)
	return s
}

type FakeSocketSession struct {
	Id           string
	onDisconnect FakeSocketHandleDisconnectFunc
	onMessage    FakeSocketHandleMessageFunc
}

func (s *FakeSocketSession) Send(data []byte) {
	s.onMessage(s, data)
}

func (s *FakeSocketSession) Disconnect() {
	s.onDisconnect(s)
}

func NewFakeConnector() (*Connector, *FakeSocket) {
	fakeSocket := &FakeSocket{}

	connector := &Connector{
		hooks: &Hooks{},
	}
	connector.clients.init()

	fakeSocket.handleConnect = func(s *FakeSocketSession) {
		client := Client{
			Id: "",
			sendMessage: func(message []byte) error {
				return fakeSocket.OnWrite(message)
			},
			properties: map[string]interface{}{},
		}

		connector.clients.Join(&client)
		s.Id = client.Id

		if connector.hooks.OnConnect != nil {
			connector.hooks.OnConnect(&client)
		}
	}

	fakeSocket.handleDisconnect = func(s *FakeSocketSession) {
		client := connector.clients.Get(s.Id)
		if connector.hooks.OnDisconnect != nil {
			connector.hooks.OnDisconnect(client)
		}
		connector.clients.Remove(client.Id)
	}

	fakeSocket.handleMessage = func(s *FakeSocketSession, data []byte) {
		client := connector.clients.Get(s.Id)
		if connector.hooks.OnMessage != nil {
			connector.hooks.OnMessage(client, data)
		}
	}

	return connector, fakeSocket
}

func SubMessage(path string) []byte {
	message := Message{
		Type:    MessageTypeSubscribe,
		Channel: path,
		Payload: nil,
	}
	data, _ := json.Marshal(message)
	return data
}

func UnsubMessage(path string) []byte {
	message := Message{
		Type:    MessageTypeUnsubscribe,
		Channel: path,
		Payload: nil,
	}
	data, _ := json.Marshal(message)
	return data
}

func TestRealtimeConnection(t *testing.T) {

	t.Run("Simple Connect Disconnect", func(t *testing.T) {
		fakeConnector, fakeSocket := NewFakeConnector()

		fakeClient := fakeSocket.NewClientConnects()

		if len(fakeConnector.clients.clients) != 1 {
			t.Errorf("len(fakeConnector.clients.clients) = %d, want %d", len(fakeConnector.clients.clients), 1)
			return
		}

		if fakeConnector.clients.Get(fakeClient.Id) == nil {
			t.Errorf("fakeConnector.clients.Get(fakeClient.Id) = nil, want *FakeSocketSession")
		}

		fakeClient.Disconnect()

		if len(fakeConnector.clients.clients) != 0 {
			t.Errorf("len(fakeConnector.clients.clients) = %d, want %d", len(fakeConnector.clients.clients), 0)
			return
		}

	})

	t.Run("Handle Sub/Unsub", func(t *testing.T) {
		channelPath := "example/path/:var"
		testVar := "test"
		testChannelPath := "example/path/" + testVar
		testChannelPath2 := "example/path/blabla"
		fakeConnector, fakeSocket := NewFakeConnector()
		realtime := New(fakeConnector)

		var subContext *Context
		var unsubContext *Context

		channel := realtime.RegisterChannel(channelPath, ChannelHandlers{
			OnSubscribe: func(s *Context) {
				subContext = s
			},
			OnUnsubscribe: func(s *Context) {
				unsubContext = s
			},
		})

		fakeClient := fakeSocket.NewClientConnects()
		fakeClient.Send(SubMessage(testChannelPath))

		if !channel.IsSubscribed(testChannelPath, fakeClient.Id) {
			t.Errorf("channel.IsSubscribed(testChannelPath, fakeClient.Id) = false, want true")
			return
		}
		if channel.IsSubscribed(channelPath, fakeClient.Id) {
			t.Errorf("channel.IsSubscribed(channelPath, fakeClient.Id) = true, want false")
			return
		}
		if channel.IsSubscribed(testChannelPath2, fakeClient.Id) {
			t.Errorf("channel.IsSubscribed(testChannelPath2, fakeClient.Id) = true, want false")
			return
		}
		if subContext == nil {
			t.Errorf("subContext = nil, want *Context")
			return
		}
		if subContext.FullPath != testChannelPath {
			t.Errorf("subContext = %s, want %s", subContext.FullPath, testChannelPath)
			return
		}

		fakeClient.Send(UnsubMessage(testChannelPath))

		if channel.IsSubscribed(testChannelPath, fakeClient.Id) {
			t.Errorf("channel.IsSubscribed(testChannelPath, fakeClient.Id) = true, want false")
			return
		}
		if unsubContext == nil {
			t.Errorf("unsubContext = nil, want *Context")
			return
		}

	})

}

/*

func TestRealtimeMessaging(t *testing.T) {

	t.Run("Simple Sub/Unsub", func(t *testing.T) {
		simplePath := "example/path/test"
		clientId := "123789"

		realtime := New()

	})

	t.Run("Simple Case", func(t *testing.T) {
		simplePath := "example/path/test"
		clientId := "123789"

		var resContext *Context
		var resMessage *Message

		store := ChannelStore{}
		store.init()
		channel := store.Register(simplePath, ChannelHandlers{
			OnMessage: func(s *Context, message *Message) {
				resContext = s
				resMessage = message
			},
		})
		store.Subscribe(&Client{Id: clientId}, simplePath)
		channel.
	})

}

*/
