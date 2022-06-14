package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownload_noContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	mockDaoWrapper := dao.DaoWrapper{}

	configGinDownloadRouter(r, mockDaoWrapper)

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDownload_notLoggedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: nil})
	})

	mockDaoWrapper := dao.DaoWrapper{}

	configGinDownloadRouter(r, mockDaoWrapper)

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDownload_fileDoesntExist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name: "Admin",
			Role: model.AdminType,
		}})
	})

	ctrl := gomock.NewController(t)
	fileDao := mock_dao.NewMockFileDao(ctrl)

	fileDao.EXPECT().GetFileById(gomock.Eq(fileId)).Return(model.File{}, errors.New("")).AnyTimes()

	mockDaoWrapper := dao.DaoWrapper{FileDao: fileDao}

	configGinDownloadRouter(r, mockDaoWrapper)

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDownload_downloadsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"
	streamId := (uint)(1234)
	courseId := (uint)(4321)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name:                "Hansi",
			Role:                model.StudentType,
			AdministeredCourses: []model.Course{},
		}})
	})

	// file mock
	fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
	fileMock.EXPECT().GetFileById(gomock.Eq(fileId)).Return(model.File{
		StreamID: streamId,
		Path:     "/file",
	}, nil).AnyTimes()

	// streams mock
	streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
		CourseID: courseId,
	}, nil).AnyTimes()

	// course mock
	courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{
		UserID:           1, // User defined above has ID 0
		DownloadsEnabled: false,
	}, nil).AnyTimes()

	configGinDownloadRouter(r, dao.DaoWrapper{
		FileDao:    fileMock,
		StreamsDao: streamsMock,
		CoursesDao: courseMock,
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDownload_fileNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"
	streamId := (uint)(1234)
	courseId := (uint)(4321)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name: "Admin",
			Role: model.AdminType,
		}})
	})

	// file mock
	fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
	fileMock.EXPECT().GetFileById(gomock.Eq(fileId)).Return(model.File{
		StreamID: streamId,
		Path:     "/file",
	}, nil).AnyTimes()

	// streams mock
	streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
		CourseID: courseId,
	}, nil).AnyTimes()

	// course mock
	courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{
		UserID:           1, // User defined above has ID 0
		DownloadsEnabled: false,
	}, nil).AnyTimes()

	configGinDownloadRouter(r, dao.DaoWrapper{
		FileDao:    fileMock,
		StreamsDao: streamsMock,
		CoursesDao: courseMock,
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDownload_success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"
	filePath := "/tmp/download_test"
	fileContent := "hello"
	streamId := (uint)(1234)
	courseId := (uint)(4321)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	// Middleware to set Mock-TUMLiveContext
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
			Name: "Admin",
			Role: model.AdminType,
		}})
	})

	// create file with content to read
	err := os.WriteFile(filePath, []byte(fileContent), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(filePath)

	// file mock
	fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
	fileMock.EXPECT().GetFileById(gomock.Eq(fileId)).Return(model.File{
		StreamID: streamId,
		Path:     filePath,
	}, nil).AnyTimes()

	// streams mock
	streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).Return(model.Stream{
		CourseID: courseId,
	}, nil).AnyTimes()

	// course mock
	courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	courseMock.EXPECT().GetCourseById(gomock.Any(), courseId).Return(model.Course{
		UserID:           1, // User defined above has ID 0
		DownloadsEnabled: true,
	}, nil).AnyTimes()

	configGinDownloadRouter(r, dao.DaoWrapper{
		FileDao:    fileMock,
		StreamsDao: streamsMock,
		CoursesDao: courseMock,
	})

	c.Request, _ = http.NewRequest(http.MethodGet, "/api/download/"+fileId, nil)
	r.ServeHTTP(w, c.Request)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, fileContent, w.Body.String())
}
