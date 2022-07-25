package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"log"
	"net/http"
	"os"
	"testing"
)

func DownloadRouterWrapper(r *gin.Engine) {
	configGinDownloadRouter(r, dao.DaoWrapper{})
}

func TestDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"
	streamId := (uint)(1234)
	courseId := (uint)(4321)
	filePath := "/tmp/download_test"
	fileContent := []byte("hello123")
	url := fmt.Sprintf("/api/download/%s", fileId)

	// create file with content to read
	err := os.WriteFile(filePath, []byte(fileContent), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(filePath)

	t.Run("/download/:id", func(t *testing.T) {
		gomino.TestCases{
			"GET[no context]": {
				Router:       DownloadRouterWrapper,
				Method:       http.MethodGet,
				Url:          url,
				ExpectedCode: http.StatusInternalServerError,
			},
			"GET[not logged in]": {
				Router:       DownloadRouterWrapper,
				Method:       http.MethodGet,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextEmpty),
				ExpectedCode: http.StatusForbidden,
			},
			"GET[file doesnt exist]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						FileDao: func() dao.FileDao {
							fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
							fileMock.
								EXPECT().
								GetFileById(gomock.Eq(fileId)).
								Return(model.File{}, errors.New("")).
								AnyTimes()
							return fileMock
						}(),
					}
					configGinDownloadRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[Downloads disabled]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						FileDao: func() dao.FileDao {
							fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
							fileMock.
								EXPECT().
								GetFileById(gomock.Eq(fileId)).
								Return(model.File{StreamID: streamId, Path: "/file"}, nil).
								AnyTimes()
							return fileMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).
								Return(model.Stream{CourseID: courseId}, nil).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseById(gomock.Any(), courseId).
								Return(model.Course{UserID: 1, DownloadsEnabled: false}, nil).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextStudent),
				ExpectedCode: http.StatusForbidden,
			},
			"GET[File not found]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						FileDao: func() dao.FileDao {
							fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
							fileMock.
								EXPECT().
								GetFileById(gomock.Eq(fileId)).
								Return(model.File{StreamID: streamId, Path: "/file"}, nil).
								AnyTimes()
							return fileMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).
								Return(model.Stream{CourseID: courseId}, nil).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseById(gomock.Any(), courseId).
								Return(model.Course{UserID: 1, DownloadsEnabled: true}, nil).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadRouter(r, wrapper)
				},
				Method:       http.MethodGet,
				Url:          url,
				Middlewares:  testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode: http.StatusNotFound,
			},
			"GET[success-download]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						FileDao: func() dao.FileDao {
							fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
							fileMock.
								EXPECT().
								GetFileById(gomock.Eq(fileId)).
								Return(model.File{StreamID: streamId, Path: filePath}, nil).
								AnyTimes()
							return fileMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).
								Return(model.Stream{CourseID: courseId}, nil).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseById(gomock.Any(), courseId).
								Return(model.Course{UserID: 1, DownloadsEnabled: true}, nil).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              url,
				Middlewares:      testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: fileContent,
			},
			"GET[success-static]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						FileDao: func() dao.FileDao {
							fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
							fileMock.
								EXPECT().
								GetFileById(gomock.Eq(fileId)).
								Return(model.File{StreamID: streamId, Path: filePath}, nil).
								AnyTimes()
							return fileMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamId)).
								Return(model.Stream{CourseID: courseId}, nil).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							courseMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							courseMock.
								EXPECT().
								GetCourseById(gomock.Any(), courseId).
								Return(model.Course{}, nil).
								AnyTimes()
							return courseMock
						}(),
					}
					configGinDownloadRouter(r, wrapper)
				},
				Method:           http.MethodGet,
				Url:              url + "?type=static",
				Middlewares:      testutils.TUMLiveMiddleware(testutils.TUMLiveContextAdmin),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: fileContent,
			},
		}.Run(t, testutils.Equal)
	})
}
