package api

import (
	"context"
	"testing"

	"github.com/TUM-Dev/gocast/worker/cfg"
	"github.com/TUM-Dev/gocast/worker/pb"
)

var mockServer = server{}

func setup() {
	cfg.WorkerID = "123"
}

func TestServer_RequestStream(t *testing.T) {
	setup()
	_, err := mockServer.RequestStream(context.Background(), &pb.StreamRequest{
		WorkerId: "234",
	})
	if err == nil {
		t.Errorf("Request with wrong WorkerID should be rejected")
		return
	}
}
