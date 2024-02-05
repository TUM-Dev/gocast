package ServerInterface

import "github.com/tum-dev/gocast/runner/protobuf"

type ServerInf interface {
	NotifyStreamStarted(started *protobuf.StreamStarted) protobuf.Status
}
