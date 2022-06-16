package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"testing"
)

func TestStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/stream/:streamID", func(t *testing.T) {
		course := testutils.CourseFPV
		stream := testutils.StreamFPVLive
		response := gin.H{
			"course":      course.Name,
			"courseID":    course.ID,
			"streamID":    stream.ID,
			"name":        stream.Name,
			"description": stream.Description,
			"start":       stream.Start,
			"end":         stream.End,
			"ingest":      fmt.Sprintf("%sstream?secret=%s", tools.Cfg.IngestBase, stream.StreamKey),
			"live":        stream.LiveNow,
			"vod":         stream.Recording}

		url := fmt.Sprintf("/api/stream/%d", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"success": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
	t.Run("GET/api/stream/:streamID/pause", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/pause", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"GetLectureHallByID returns error": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					LectureHallsDao: testutils.GetLectureHallMockError(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
	t.Run("GET/api/stream/:streamID/end", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/end", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			/*"success discard": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url + "?discard=true",
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:  testutils.GetStreamMock(t),
					CoursesDao:  testutils.GetCoursesMock(t),
					ProgressDao: testutils.GetProgressMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},
			"success no discard": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:  testutils.GetStreamMock(t),
					CoursesDao:  testutils.GetCoursesMock(t),
					ProgressDao: testutils.GetProgressMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},*/
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
	t.Run("GET/api/stream/:streamID/visibility", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/visibility", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"invalid body": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"success": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(gin.H{"private": false})).([]byte)),
				ExpectedCode:   http.StatusOK,
			},
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
}

func TestStreamVideoSections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET/api/stream/:streamID/sections", func(t *testing.T) {
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
			"GetByStreamId returns error": {
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
			"success": {
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
	t.Run("POST/api/stream/:streamID/sections", func(t *testing.T) {
		request := []model.VideoSection{
			{
				Description:  "Seidlwave",
				StartHours:   0,
				StartMinutes: 1,
				StartSeconds: 0,
				StreamID:     testutils.StreamFPVLive.ID,
			},
		}
		url := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"Not Admin": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"Invalid Body": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"Create returns error": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMockError(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode:   http.StatusOK,
			},
		}

		testCases.Run(t, configGinStreamRestRouter)
	})
	t.Run("DELETE/api/stream/:streamID/sections", func(t *testing.T) {
		section := testutils.StreamFPVLive.VideoSections[0]
		baseUrl := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"Not Admin": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"Invalid ID": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%s", baseUrl, "abc"),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"Delete returns error": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMockError(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					VideoSectionDao: testutils.GetVideoSectionMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
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
