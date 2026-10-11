package apiv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

// bookmarkRecorder keeps what the gateway handed to the add/update RPCs.
type bookmarkRecorder struct {
	protobuf.UnimplementedStreamServiceServer
	added   *protobuf.AddBookmarkRequest
	updated *protobuf.UpdateBookmarkRequest
}

func (r *bookmarkRecorder) AddBookmark(_ context.Context, req *protobuf.AddBookmarkRequest) (*protobuf.AddBookmarkResponse, error) {
	r.added = req
	return &protobuf.AddBookmarkResponse{}, nil
}

func (r *bookmarkRecorder) UpdateBookmark(_ context.Context, req *protobuf.UpdateBookmarkRequest) (*protobuf.UpdateBookmarkResponse, error) {
	r.updated = req
	return &protobuf.UpdateBookmarkResponse{}, nil
}

// The add and update bookmark routes must read their fields from the JSON body: without
// `body: "*"` in the proto the gateway silently takes them from the query string and
// drops the body, creating an empty bookmark at 0:00.
func TestBookmarkRoutesReadTheJSONBody(t *testing.T) {
	rec := &bookmarkRecorder{}
	mux := runtime.NewServeMux()
	if err := protobuf.RegisterStreamServiceHandlerServer(context.Background(), mux, rec); err != nil {
		t.Fatal(err)
	}

	do := func(method, path, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s %s: status %d: %s", method, path, w.Code, w.Body.String())
		}
	}

	do(http.MethodPost, "/bookmarks", `{"streamId":7,"description":"new","hours":1,"seconds":5}`)
	if got := rec.added; got == nil || got.StreamId != 7 || got.Description != "new" || got.Hours != 1 || got.Minutes != 0 || got.Seconds != 5 {
		t.Errorf("AddBookmark got %v", got)
	}

	do(http.MethodPut, "/bookmarks/3", `{"description":"renamed","minutes":20}`)
	if got := rec.updated; got == nil || got.BookmarkId != 3 || got.Description != "renamed" || got.Minutes != 20 {
		t.Errorf("UpdateBookmark got %v", got)
	}
}
