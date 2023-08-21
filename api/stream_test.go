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
	"github.com/matthiasreumann/gomino"
	"gorm.io/gorm"
	"net/http"
	"os"
	"testing"
)

func StreamRouterWrapper(r *gin.Engine) {
	configGinStreamRestRouter(r, dao.DaoWrapper{})
}

func StreamDefaultRouter(t *testing.T) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		wrapper := dao.DaoWrapper{
			StreamsDao: testutils.GetStreamMock(t),
			CoursesDao: testutils.GetCoursesMock(t),
		}
		configGinStreamRestRouter(r, wrapper)
	}
}

func TestStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
		gomino.TestCases{
			"GetCurrentLive returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return([]model.Stream{}, errors.New(""))
							return streamsMock
						}(),
						TokenDao: testutils.GetTokenMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"GetCourseById returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
			"GetLectureHallByID returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.EXPECT().GetLectureHallByID(gomock.Any()).Return(model.LectureHall{}, errors.New(""))
							return lectureHallMock
						}(),
						TokenDao: testutils.GetTokenMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: responseLHError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao:      testutils.GetStreamMock(t),
						CoursesDao:      testutils.GetCoursesMock(t),
						LectureHallsDao: testutils.GetLectureHallMock(t),
						TokenDao:        testutils.GetTokenMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
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
			"ingest":      fmt.Sprintf("%s%s-%d?secret=%s", tools.Cfg.IngestBase, course.Slug, stream.ID, stream.StreamKey),
			"live":        stream.LiveNow,
			"vod":         stream.Recording}

		url := fmt.Sprintf("/api/stream/%d", testutils.StreamFPVLive.ID)

		gomino.TestCases{
			"no context": {
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			}}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("GET/api/stream/:streamID/end", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/end", testutils.StreamFPVLive.ID)
		gomino.TestCases{
			"no context": {
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
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
				ExpectedCode:   http.StatusOK,},*/
		}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("PATCH/api/stream/:streamID/visibility", func(t *testing.T) {
		url := fmt.Sprintf("/api/stream/%d/visibility", testutils.StreamFPVLive.ID)
		gomino.TestCases{
			"no context": {
				Method:       http.MethodPatch,
				Url:          url,
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"ToggleVisibility returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         gin.H{"private": false},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
						AuditDao: func() dao.AuditDao {
							mock := mock_dao.NewMockAuditDao(gomock.NewController(t))
							mock.EXPECT().Create(gomock.Any()).AnyTimes().Return(nil)
							return mock
						}(),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         gin.H{"private": false},
				ExpectedCode: http.StatusOK,
			}}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodPatch).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestStreamVideoSections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("GET/api/stream/:streamID/sections", func(t *testing.T) {
		// generate same response as in handler

		url := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		gomino.TestCases{
			"GetByStreamId returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []gin.H{},
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.StreamFPVLive.VideoSections,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
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
		gomino.TestCases{
			"Not Admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"Invalid Body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"Create returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"GetByStreamId returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			/*"success": {
				Method: http.MethodPost,
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
		}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("PUT/api/stream/:streamID/sections/:id", func(t *testing.T) {
		section := testutils.StreamFPVLive.VideoSections[0]
		baseUrl := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		url := fmt.Sprintf("%s/%d", baseUrl, section.ID)

		request := UpdateVideoSectionRequest{
			Description: "Graph algorithms",
		}

		update := model.VideoSection{
			Model:       gorm.Model{ID: section.ID},
			Description: request.Description,
		}

		gomino.TestCases{
			"Not Admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"Invalid ID": {
				Url:          fmt.Sprintf("%s/%s", baseUrl, "abc"),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"Invalid Body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Body:         nil,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"Update fails": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
						VideoSectionDao: func() dao.VideoSectionDao {
							sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
							sectionMock.
								EXPECT().
								Update(&update).
								Return(errors.New("")).
								AnyTimes()
							return sectionMock
						}(),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Body:         request,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
						VideoSectionDao: func() dao.VideoSectionDao {
							sectionMock := mock_dao.NewMockVideoSectionDao(gomock.NewController(t))
							sectionMock.
								EXPECT().
								Update(&update).
								Return(nil).
								AnyTimes()
							return sectionMock
						}(),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Body:         request,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			},
		}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodPut).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("DELETE/api/stream/:streamID/sections/:id", func(t *testing.T) {
		section := testutils.StreamFPVLive.VideoSections[0]
		baseUrl := fmt.Sprintf("/api/stream/%d/sections", testutils.StreamFPVLive.ID)
		url := fmt.Sprintf("%s/%d", baseUrl, section.ID)
		gomino.TestCases{
			"Not Admin": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"Invalid ID": {
				Url:          fmt.Sprintf("%s/%s", baseUrl, "abc"),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"Get returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"Delete returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
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
		}.
			Router(StreamDefaultRouter(t)).
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestAttachments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/stream/:streamID/files", func(t *testing.T) {
		os.Create("/tmp/test.txt")
		defer os.Remove("/tmp/test.txt")

		_, w := gomino.NewMultipartFormData("file", "/tmp/test.txt")

		endpoint := fmt.Sprintf("/api/stream/%d/files", testutils.StreamFPVLive.ID)
		gomino.TestCases{
			"no context": {
				ExpectedCode: http.StatusInternalServerError,
			},
			"not Admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid type": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Url:          endpoint + "?type=abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"type url, missing file_url": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Url:          endpoint + "?type=url",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"type file, missing file parameter": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Url:          endpoint + "?type=file",
				ContentType:  w.FormDataContentType(),
				Body:         "",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
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
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Url:          endpoint + "?type=url",
				ContentType:  "application/x-www-form-urlencoded",
				Body:         "file_url=https://storage.com/test.txt",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"type url, success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Url:          endpoint + "?type=url",
				ContentType:  "application/x-www-form-urlencoded",
				Body:         "file_url=https://storage.com/test.txt",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Router(StreamRouterWrapper).
			Method(http.MethodPost).
			Url(endpoint).
			Run(t, testutils.Equal)
	})

	t.Run("DELETE/api/stream/:streamID/files/:fid", func(t *testing.T) {
		testFile := testutils.Attachment
		testFileNotExists := testutils.AttachmentInvalidPath
		url := fmt.Sprintf("/api/stream/%d/files/%d", testutils.StreamFPVLive.ID, testFile.ID)
		gomino.TestCases{
			"no context": {
				Router:       StreamRouterWrapper,
				Method:       http.MethodDelete,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"GetFileById returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"non existing file": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"DeleteFile returns error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
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
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
				Before: func() {
					_, _ = os.Create(testFile.Path)
				},
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: testutils.GetStreamMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
						FileDao:    testutils.GetFileMock(t),
					}
					configGinStreamRestRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				Before: func() {
					_, _ = os.Create(testFile.Path)
				},
			}}.
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)

		// After a successful run, the file /tmp/test.txt should be deleted
		if _, err := os.Stat(testFile.Path); !errors.Is(err, os.ErrNotExist) {
			t.Fail()
			_ = os.Remove(testFile.Path) // Then cleanup
		}
	})
}

func TestSubtitles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	endpoint := fmt.Sprintf("/api/stream/%d/subtitles/en", testutils.StreamFPVLive.ID)
	gomino.TestCases{
		"no context": {
			ExpectedCode: http.StatusInternalServerError,
		},
		"subtitles not found": {
			Router: func(r *gin.Engine) {
				wrapper := dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					SubtitlesDao: func() dao.SubtitlesDao {
						subMock := mock_dao.NewMockSubtitlesDao(gomock.NewController(t))
						subMock.
							EXPECT().
							GetByStreamIDandLang(gomock.Any(), testutils.StreamFPVLive.ID, "en").
							Return(model.Subtitles{}, gorm.ErrRecordNotFound)
						return subMock
					}(),
				}
				configGinStreamRestRouter(r, wrapper)
			},
			Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
			ExpectedCode: http.StatusNotFound,
		},
		"internal error": {
			Router: func(r *gin.Engine) {
				wrapper := dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					SubtitlesDao: func() dao.SubtitlesDao {
						subMock := mock_dao.NewMockSubtitlesDao(gomock.NewController(t))
						subMock.
							EXPECT().
							GetByStreamIDandLang(gomock.Any(), testutils.StreamFPVLive.ID, "en").
							Return(model.Subtitles{}, errors.New(""))
						return subMock
					}(),
				}
				configGinStreamRestRouter(r, wrapper)
			},
			Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
			ExpectedCode: http.StatusInternalServerError,
		},
		"success": {
			Router: func(r *gin.Engine) {
				wrapper := dao.DaoWrapper{
					StreamsDao: testutils.GetStreamMock(t),
					CoursesDao: testutils.GetCoursesMock(t),
					SubtitlesDao: func() dao.SubtitlesDao {
						subMock := mock_dao.NewMockSubtitlesDao(gomock.NewController(t))
						subMock.
							EXPECT().
							GetByStreamIDandLang(gomock.Any(), testutils.StreamFPVLive.ID, "en").
							Return(testutils.SubtitlesFPVLive, nil)
						return subMock
					}(),
				}
				configGinStreamRestRouter(r, wrapper)
			},
			Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
			ExpectedCode:     http.StatusOK,
			ExpectedResponse: testutils.SubtitlesFPVLive.Content,
		}}.
		Router(StreamDefaultRouter(t)).
		Method(http.MethodGet).
		Url(endpoint).
		Run(t, testutils.Equal)
}
