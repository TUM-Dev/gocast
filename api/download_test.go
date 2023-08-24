package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
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
	fileContent := []byte("hello123") // TODO: Bug in gomino (fix after update)
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"GET[not logged in]": {
				Router:       DownloadRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
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
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
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
				Url:              url + "?type=static",
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: fileContent,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}
