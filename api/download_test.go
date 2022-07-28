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
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"log"
	"net/http"
	"os"
	"testing"
)

func TestDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fileId := "1"
	streamId := (uint)(1234)
	courseId := (uint)(4321)
	filePath := "/tmp/download_test"
	fileContent := "hello123"
	url := fmt.Sprintf("/api/download/%s", fileId)

	// create file with content to read
	err := os.WriteFile(filePath, []byte(fileContent), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(filePath)

	t.Run("/download/:id", func(t *testing.T) {
		testutils.TestCases{
			"GET[no context]": {
				Method:         "GET",
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"GET[not logged in]": {
				Method:         "GET",
				Url:            url,
				TumLiveContext: &tools.TUMLiveContext{User: nil},
				ExpectedCode:   http.StatusForbidden,
			},
			"GET[file doesnt exist]": {
				Method: "GET",
				Url:    url,
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				DaoWrapper: dao.DaoWrapper{
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(gomock.Eq(fileId)).
							Return(model.File{}, errors.New("")).
							AnyTimes()
						return fileMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[Downloads disabled]": {
				Method: "GET",
				Url:    url,
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.StudentType,
				}},
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode: http.StatusForbidden,
			},
			"GET[File not found]": {
				Method: "GET",
				Url:    url,
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode: http.StatusNotFound,
			},
			"GET[success-download]": {
				Method: "GET",
				Url:    url,
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []byte(fileContent),
			},
			"GET[success-static]": {
				Method: "GET",
				Url:    url + "?type=static",
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []byte(fileContent),
			},
		}.Run(t, configGinDownloadRouter)
	})
}
