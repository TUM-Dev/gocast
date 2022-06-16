package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestVideoSections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("/api/stream/:streamID/sections", func(t *testing.T) {
		// generate same response as in handler
		response := []gin.H{}
		for _, section := range testutils.StreamFPVLive.VideoSections {
			response = append(response, gin.H{
				"ID":                section.ID,
				"startHours":        section.StartHours,
				"startMinutes":      section.StartMinutes,
				"startSeconds":      section.StartSeconds,
				"description":       section.Description,
				"friendlyTimestamp": section.TimestampAsString(),
				"streamID":          section.StreamID,
			})
		}

		url := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"GET[GetByStreamId returns error]": {
				Method: "GET",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMockError(t),
				},
				TumLiveContext:   &testutils.TUMLiveContextStudent,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal([]gin.H{})).([]byte),
			},
			"GET[success]": {
				Method: "GET",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMock(t),
				},
				TumLiveContext:   &testutils.TUMLiveContextStudent,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}

		testCases.Run(t, configGinStreamRestRouter)
	})
}

/*func TestAttachments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("/api/stream/:streamID/files", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/files", testutils.StreamFPVLive.ID)

		testCases := testutils.TestCases{
			"POST[no context]": {
				Method:         "POST",
				Url:            url,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"POST[not Admin]": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVNotLive, nil).AnyTimes()
						return streamsMock
					}(),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"POST[type url, missing file_url]": {
				Method: "POST",
				Url:    url + "?type=url",
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVNotLive, nil).AnyTimes()
						return streamsMock
					}(),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"POST[NewFile returns error]": {
				Method: "POST",
				Url:    url + "?type=url",
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVNotLive, nil).AnyTimes()
						return streamsMock
					}(),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.EXPECT().NewFile(gomock.Any()).Return(errors.New(""))
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"POST[type url, success]": {
				Method: "POST",
				Url:    url + "?type=url",
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVNotLive, nil).AnyTimes()
						return streamsMock
					}(),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
							Return(testutils.CourseFPV, nil).
							AnyTimes()
						return coursesMock
					}(),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.EXPECT().NewFile(model.File{
							StreamID: testutils.StreamFPVLive.ID,
							Path:     "https://files.tum.de/txt.txt",
							Filename: "txt.txt",
							Type:     model.FILETYPE_ATTACHMENT,
						}).Return(nil)
						return fileMock
					}(),
				},
				Body: bytes.NewBuffer(
					testutils.NewFormBody(map[string]string{"file_url": "https://files.tum.de/txt.txt"})),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},
		}

		testCases.Run(t, configGinStreamRestRouter)
	})
}*/
