package apiv2

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

type progressRecorder struct {
	protobuf.UnimplementedStreamServiceServer
	got *protobuf.UpdateProgressRequest
}

func (r *progressRecorder) UpdateProgress(_ context.Context, req *protobuf.UpdateProgressRequest) (*protobuf.StreamProgress, error) {
	r.got = req
	return &protobuf.StreamProgress{}, nil
}

// The three states of `watched` must survive the JSON gateway: absent (a plain progress
// report), true (mark) and false (unmark). With a plain bool the first and last were
// indistinguishable, which is why unmarking never worked.
func TestUpdateProgressRouteKeepsWatchedPresence(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want *bool
	}{
		"absent": {`{"progress": 0.5}`, nil},
		"true":   {`{"progress": 1, "watched": true}`, boolPtr(true)},
		"false":  {`{"progress": 0, "watched": false}`, boolPtr(false)},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &progressRecorder{}
			mux := runtime.NewServeMux()
			if err := protobuf.RegisterStreamServiceHandlerServer(context.Background(), mux, rec); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPatch, "/progress/7", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			switch {
			case tc.want == nil && rec.got.Watched != nil:
				t.Errorf("watched = %v, want absent", *rec.got.Watched)
			case tc.want != nil && (rec.got.Watched == nil || *rec.got.Watched != *tc.want):
				t.Errorf("watched = %v, want %v", rec.got.Watched, *tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func TestUpdateProgress(t *testing.T) {
	viewer := &model.User{Model: gorm.Model{ID: 3}, Name: "Vera"}
	ctx := context.WithValue(context.Background(), callerKey{}, &caller{user: viewer})

	// setup expects exactly one save of stream 7 for the viewer, passes the watched
	// argument it got to `saved`, and answers the read-back with `stored`.
	setup := func(t *testing.T, stored model.StreamProgress, saved **bool) *API {
		ctrl := gomock.NewController(t)
		streams := mock_dao.NewMockStreamsDao(ctrl)
		streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5}, nil)
		courses := mock_dao.NewMockCoursesDao(ctrl)
		courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).Return(model.Course{Model: gorm.Model{ID: 5}, Visibility: "public"}, nil)
		progresses := mock_dao.NewMockProgressDao(ctrl)
		progresses.EXPECT().SaveProgress(model.StreamProgress{StreamID: 7, UserID: 3, Progress: 0.5}, gomock.Any()).
			DoAndReturn(func(_ model.StreamProgress, w *bool) error { *saved = w; return nil })
		progresses.EXPECT().LoadProgress(uint(3), []uint{7}).Return([]model.StreamProgress{stored}, nil)
		return &API{
			dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses, ProgressDao: progresses},
			log: slog.Default(),
		}
	}

	for name, tc := range map[string]struct {
		watched *bool
		stored  bool
	}{
		"a plain report leaves the flag alone": {nil, true},
		"true marks":                           {boolPtr(true), true},
		"false clears":                         {boolPtr(false), false},
	} {
		t.Run(name, func(t *testing.T) {
			var saved *bool
			api := setup(t, model.StreamProgress{StreamID: 7, UserID: 3, Progress: 0.5, Watched: tc.stored}, &saved)

			resp, err := api.UpdateProgress(ctx, &protobuf.UpdateProgressRequest{StreamId: 7, Progress: 0.5, Watched: tc.watched})
			if err != nil {
				t.Fatalf("UpdateProgress: %v", err)
			}
			if (saved == nil) != (tc.watched == nil) || (saved != nil && *saved != *tc.watched) {
				t.Errorf("DAO got watched %v, want %v", saved, tc.watched)
			}
			// The answer is the stored state, not an echo of the request.
			if resp.Watched != tc.stored {
				t.Errorf("response watched = %v, want %v", resp.Watched, tc.stored)
			}
		})
	}
}
