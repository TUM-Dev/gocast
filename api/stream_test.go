package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"net/http"
	"os"
	"testing"
)

func TestStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	t.Run("GET/api/stream/live", func(t *testing.T) {
		response := []liveStreamDto{
			{
				ID:          testutils.StreamFPVLive.ID,
				CourseName:  testutils.CourseFPV.Name,
				LectureHall: testutils.LectureHall.Name,
				COMB:        testutils.StreamFPVLive.PlaylistUrl,
				PRES:        testutils.StreamFPVLive.PlaylistUrlPRES,
				CAM:         testutils.StreamFPVLive.PlaylistUrlCAM,
				End:         testutils.StreamFPVLive.End,
			},
		}
		responseLHError := []liveStreamDto{
			{
				ID:          testutils.StreamFPVLive.ID,
				CourseName:  testutils.CourseFPV.Name,
				LectureHall: "Selfstream",
				COMB:        testutils.StreamFPVLive.PlaylistUrl,
				PRES:        testutils.StreamFPVLive.PlaylistUrlPRES,
				CAM:         testutils.StreamFPVLive.PlaylistUrlCAM,
				End:         testutils.StreamFPVLive.End,
			},
		}
		url := fmt.Sprintf("/api/stream/live?token=%s", testutils.AdminToken.Token)
		testCases := testutils.TestCases{
			"GetCurrentLive returns error": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return([]model.Stream{}, errors.New(""))
						return streamsMock
					}(),
					TokenDao: testutils.GetTokenMock(t),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"GetCourseById returns error": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.
							EXPECT().
							GetCourseById(gomock.Any(), gomock.Any()).
							Return(testutils.CourseFPV, errors.New("")).
							AnyTimes()
						return coursesMock
					}(),
					LectureHallsDao: testutils.GetLectureHallMock(t),
					TokenDao:        testutils.GetTokenMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
			"GetLectureHallByID returns error": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.EXPECT().GetLectureHallByID(gomock.Any()).Return(model.LectureHall{}, errors.New(""))
						return lectureHallMock
					}(),
					TokenDao: testutils.GetTokenMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(responseLHError)).([]byte),
			},
			"success": {
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao:      testutils.GetStreamMock(t),
					CoursesDao:      testutils.GetCoursesMock(t),
					LectureHallsDao: testutils.GetLectureHallMock(t),
					TokenDao:        testutils.GetTokenMock(t),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(response)).([]byte),
			},
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
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
	t.Run("PATCH/api/stream/:streamID/visibility", func(t *testing.T) {
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
			"ToggleVisibility returns error": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
							Return(testutils.StreamFPVLive, nil).AnyTimes()
						streamsMock.
							EXPECT().
							ToggleVisibility(testutils.StreamFPVLive.ID, gomock.Any()).
							Return(errors.New("")).AnyTimes()
						return streamsMock
					}(),
					AuditDao: func() dao.AuditDao {
						mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
						mock.EXPECT().Create(gomock.Any()).AnyTimes().Return(nil)
						return mock
					}(),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(gin.H{"private": false})).([]byte)),
				ExpectedCode:   http.StatusInternalServerError,
			},
			"success": testutils.TestCase{
				Method: http.MethodPatch,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					AuditDao: func() dao.AuditDao {
						mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
						mock.EXPECT().Create(gomock.Any()).AnyTimes().Return(nil)
						return mock
					}(),
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
	t.Parallel()
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
				"fileID":            section.FileID,
			})
		}

		url := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"GetByStreamId returns error": {
				Method: "GET",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							GetByStreamId(testutils.StreamFPVLive.ID).
							Return([]model.VideoSection{}, errors.New(""))
						return sectionMock
					}(),
				},
				TumLiveContext:   &testutils.TUMLiveContextStudent,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal([]gin.H{})).([]byte),
			},
			"success": {
				Method: "GET",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							GetByStreamId(testutils.StreamFPVLive.ID).
							Return(testutils.StreamFPVLive.VideoSections, nil)
						return sectionMock
					}(),
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
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Create(gomock.Any()).
							Return(errors.New(""))
						return sectionMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode:   http.StatusInternalServerError,
			},
			"GetByStreamId returns error": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Create(gomock.Any()).
							Return(nil)
						sectionMock.
							EXPECT().
							GetByStreamId(testutils.StreamFPVLive.ID).
							Return([]model.VideoSection{}, errors.New(""))
						return sectionMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode:   http.StatusInternalServerError,
			},
			/*"success": {
				Method: "POST",
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					WorkerDao: func() dao.WorkerDao {
						workerMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
						workerMock.EXPECT().GetAliveWorkers().Return([]model.Worker{testutils.Worker1})
						return workerMock
					}(),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Create(gomock.Any()).
							Return(nil)
						sectionMock.
							EXPECT().
							GetByStreamId(testutils.StreamFPVLive.ID).
							Return([]model.VideoSection{testutils.StreamFPVLive.VideoSections[0]}, nil)
						return sectionMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode:   http.StatusOK,
			},*/
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
			"Get returns error": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Get(section.ID).
							Return(section, errors.New("")).
							AnyTimes()
						return sectionMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"GetFileById returns error": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Get(section.ID).
							Return(section, nil).
							AnyTimes()
						return sectionMock
					}(),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", section.ID)).
							Return(model.File{}, errors.New("")).
							AnyTimes()
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusNotFound,
			},
			"Delete returns error": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Get(section.ID).
							Return(section, nil).
							AnyTimes()
						sectionMock.
							EXPECT().
							Delete(gomock.Any()).
							Return(errors.New(""))
						return sectionMock
					}(),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", section.ID)).
							Return(model.File{}, nil).
							AnyTimes()
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			/*"success": {
				Method: http.MethodDelete,
				Url:    fmt.Sprintf("%s/%d", baseUrl, section.ID),
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					WorkerDao: func() dao.WorkerDao {
						workerMock := mock_dao.NewMockWorkerDao(gomock.NewController(t))
						workerMock.EXPECT().GetAliveWorkers().Return([]model.Worker{testutils.Worker1})
						return workerMock
					}(),
					VideoSectionDao: func() dao.VideoSectionDao {
						sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
						sectionMock.
							EXPECT().
							Get(section.ID).
							Return(section, nil).
							AnyTimes()
						sectionMock.
							EXPECT().
							Delete(gomock.Any()).
							Return(nil)
						return sectionMock
					}(),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", section.ID)).
							Return(model.File{}, nil).
							AnyTimes()
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusAccepted,
			},*/
		}
		testCases.Run(t, configGinStreamRestRouter)
	})
}

func TestAttachments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/stream/:streamID/files", func(t *testing.T) {
		os.Create("/tmp/test.txt")
		defer os.Remove("/tmp/test.txt")

		_, w := testutils.NewMultipartFormData("file", "/tmp/test.txt")

		endpoint := fmt.Sprintf("/api/stream/%d/files", testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            endpoint,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not Admin": {
				Method: http.MethodPost,
				Url:    endpoint,
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
			"invalid type": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=abc",
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
			"type url, missing file_url": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=url",
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
			"type file, missing file parameter": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=file",
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
				ContentType:    w.FormDataContentType(),
				Body:           bytes.NewBufferString(""),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			// The Test below currently fails since the tester can't mkdir
			/*"type file, success": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=file",
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
						fileMock.
							EXPECT().
							NewFile(gomock.Any()).
							Return(nil)
						return fileMock
					}(),
				},
				ContentType:    w.FormDataContentType(),
				Body:           bytes.NewBuffer(formData.Bytes()),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},*/
			"type url, NewFile returns error": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=url",
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
				ContentType:    "application/x-www-form-urlencoded",
				Body:           bytes.NewBufferString("file_url=https://storage.com/test.txt"),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"type url, success": {
				Method: http.MethodPost,
				Url:    endpoint + "?type=url",
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
						fileMock.EXPECT().NewFile(&model.File{
							StreamID: testutils.StreamFPVLive.ID,
							Path:     "https://storage.com/test.txt",
							Filename: "test.txt",
							Type:     model.FILETYPE_ATTACHMENT,
						}).Return(nil)
						return fileMock
					}(),
				},
				ContentType:    "application/x-www-form-urlencoded",
				Body:           bytes.NewBufferString("file_url=https://storage.com/test.txt"),
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
			},
		}

		testCases.Run(t, configGinStreamRestRouter)
	})

	t.Run("DELETE/api/stream/:streamID/files/:fid", func(t *testing.T) {
		testFile := testutils.Attachment
		testFileNotExists := testutils.AttachmentInvalidPath
		url := fmt.Sprintf("/api/stream/%d/files/%d", testutils.StreamFPVLive.ID, testFile.ID)
		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method:         http.MethodDelete,
				Url:            url,
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"not admin": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextStudent,
				ExpectedCode:   http.StatusForbidden,
			},
			"GetFileById returns error": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", testFile.ID)).
							Return(model.File{}, errors.New(""))
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"non existing file": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", testFileNotExists.ID)).
							Return(testFileNotExists, nil)
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"DeleteFile returns error": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					FileDao: func() dao.FileDao {
						fileMock := mock_dao.NewMockFileDao(gomock.NewController(t))
						fileMock.
							EXPECT().
							GetFileById(fmt.Sprintf("%d", testFile.ID)).
							Return(testFile, nil)
						fileMock.
							EXPECT().
							DeleteFile(testFile.ID).
							Return(errors.New(""))
						return fileMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusInternalServerError,
				Before: func() {
					_, _ = os.Create(testFile.Path)
				},
			},
			"success": testutils.TestCase{
				Method: http.MethodDelete,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					FileDao:    testutils.GetFileMock(t),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusOK,
				Before: func() {
					_, _ = os.Create(testFile.Path)
				},
			},
		}
		testCases.Run(t, configGinStreamRestRouter)

		// After a successful run, the file /tmp/test.txt should be deleted
		if _, err := os.Stat(testFile.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fail()
			_ = os.Remove(testFile.Path) // Then cleanup
		}
	})
}
