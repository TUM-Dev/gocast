package apiv2

import (
	"context"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// GetLiveCourses answers without credentials, so its visibility rules are all that
// stands between an anonymous caller and an unlisted course. A context with no
// metadata is what an anonymous request looks like by the time it reaches the handler.
func TestGetLiveCoursesHidesRestrictedCoursesFromAnonymousCallers(t *testing.T) {
	course := func(id uint, visibility string) model.Course {
		return model.Course{
			Model:      gorm.Model{ID: id},
			Name:       visibility + " course",
			Slug:       visibility,
			Visibility: visibility,
			// A real owner: an unset one would make the zero-value user an admin via
			// `course.UserID == u.ID`.
			UserID: 42,
		}
	}

	courses := map[uint]model.Course{
		1: course(1, "public"),
		2: course(2, "loggedin"),
		3: course(3, "enrolled"),
		4: course(4, "hidden"),
	}

	streams := []model.Stream{
		{Model: gorm.Model{ID: 11}, CourseID: 1, LiveNow: true},
		{Model: gorm.Model{ID: 12}, CourseID: 2, LiveNow: true},
		{Model: gorm.Model{ID: 13}, CourseID: 3, LiveNow: true},
		{Model: gorm.Model{ID: 14}, CourseID: 4, LiveNow: true},
		{Model: gorm.Model{ID: 15}, CourseID: 1, LiveNow: true, Private: true},
	}

	ctrl := gomock.NewController(t)

	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return(streams, nil).Times(1)

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (model.Course, error) {
			return courses[id], nil
		}).AnyTimes()

	api := &API{
		dao: dao.DaoWrapper{StreamsDao: streamsMock, CoursesDao: coursesMock},
		log: slog.Default(),
	}

	resp, err := api.GetLiveCourses(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetLiveCourses: %v", err)
	}

	var got []string
	for _, entry := range resp.LiveCourses {
		got = append(got, entry.Course.Slug)
	}

	// Only the public course. Its private stream is dropped too: that needs course
	// admin rights of its own.
	if len(got) != 1 || got[0] != "public" {
		t.Errorf("anonymous caller saw %v, want only [public]", got)
	}
}
