package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func serveShortLink(t *testing.T, coursesDao dao.CoursesDao) *httptest.ResponseRecorder {
	t.Helper()
	ctrl := gomock.NewController(t)
	infoPageDao := mock_dao.NewMockInfoPageDao(ctrl)
	infoPageDao.EXPECT().GetBySlug("ws").Return(model.InfoPage{}, gorm.ErrRecordNotFound)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := mainRoutes{DaoWrapper: dao.DaoWrapper{CoursesDao: coursesDao, InfoPageDao: infoPageDao}}
	router.GET("/:shortLink", routes.shortLinkOrInfoPage)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws", nil))
	return rec
}

func TestShortLinkRedirectsToCurrentLecture(t *testing.T) {
	ctrl := gomock.NewController(t)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	course := model.Course{Model: gorm.Model{ID: 3}, Slug: "ws", Year: 2024, TeachingTerm: "W"}
	lecture := model.Stream{Model: gorm.Model{ID: 42}, Name: "Secret lecture title", CourseID: 3}
	coursesDao.EXPECT().GetCourseByShortLink("ws").Return(course, nil)
	coursesDao.EXPECT().GetCurrentOrNextLectureForCourse(gomock.Any(), uint(3)).Return(lecture, nil)

	rec := serveShortLink(t, coursesDao)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/w/ws/42", rec.Header().Get("Location"))
	// Access is enforced by the target; the redirect must not carry lecture data.
	assert.NotContains(t, rec.Body.String(), "Secret lecture title")
}

func TestShortLinkWithoutLectureRedirectsToCoursePage(t *testing.T) {
	ctrl := gomock.NewController(t)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	course := model.Course{Model: gorm.Model{ID: 3}, Slug: "ws", Year: 2024, TeachingTerm: "W"}
	coursesDao.EXPECT().GetCourseByShortLink("ws").Return(course, nil)
	coursesDao.EXPECT().GetCurrentOrNextLectureForCourse(gomock.Any(), uint(3)).Return(model.Stream{}, gorm.ErrRecordNotFound)

	rec := serveShortLink(t, coursesDao)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/course/2024/W/ws", rec.Header().Get("Location"))
}

func TestUnknownShortLinkIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().GetCourseByShortLink("ws").Return(model.Course{}, gorm.ErrRecordNotFound)

	rec := serveShortLink(t, coursesDao)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
