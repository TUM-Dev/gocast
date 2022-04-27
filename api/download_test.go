package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/magiconair/properties/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownload_success(t *testing.T) {
	fileId := "1234"
	var streamId uint = 1
	var courseId uint = 42
	fileDaoMock := createFileDaoMock(t, fileId, streamId)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", createMockTUMLiveContext())
	})

	streamsDaoMock := createStreamsDaoMock(t, c, streamId, courseId)
	coursesDaoMock := createCoursesDaoMock(t, c, courseId)
	configGinDownloadRouter(r, fileDaoMock, streamsDaoMock, coursesDaoMock)

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Body.String(), "")
}

func createFileDaoMock(t *testing.T, fileId string, streamId uint) *mock_dao.MockFileDao {
	ctrl := gomock.NewController(t)
	fileDaoMock := mock_dao.NewMockFileDao(ctrl)
	fileDaoMock.EXPECT().GetFileById(fileId).Return(model.File{
		StreamID: streamId,
		Path:     "/file",
	}, nil).AnyTimes()

	return fileDaoMock
}

func createStreamsDaoMock(t *testing.T, ctx *gin.Context, streamId uint, courseId uint) *mock_dao.MockStreamsDao {
	// Because the context needs to be exactly the same, we need to set it here too.
	ctx.Set("TUMLiveContext", createMockTUMLiveContext())
	ctx.Params = []gin.Param{{Key: "id", Value: "1234"}}
	ctrl := gomock.NewController(t)
	streamsDaoMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsDaoMock.EXPECT().GetStreamByID(ctx, fmt.Sprintf("%d", streamId)).Return(model.Stream{
		CourseID: courseId,
	}, nil).AnyTimes()

	return streamsDaoMock
}

func createCoursesDaoMock(t *testing.T, ctx *gin.Context, courseId uint) *mock_dao.MockCoursesDao {
	// Because the context needs to be exactly the same, we need to set it here too.
	// TODO: Context is different because gin loads everything into the context, and i cant set all the values right now.
	ctx.Set("TUMLiveContext", createMockTUMLiveContext())
	ctx.Params = []gin.Param{{Key: "id", Value: "1234"}}
	ctrl := gomock.NewController(t)
	coursesDaoMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesDaoMock.EXPECT().GetCourseById(ctx, courseId).Return(model.Course{
		DownloadsEnabled: true,
		Visibility:       "public",
	}, nil).AnyTimes()

	return coursesDaoMock
}
