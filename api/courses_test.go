package api

import (
	"errors"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"github.com/dgraph-io/ristretto"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/matthiasreumann/gomino"
	"github.com/u2takey/go-utils/uuid"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"testing"
	"time"
)

func CourseRouterWrapper(r *gin.Engine) {
	configGinCourseRouter(r, dao.DaoWrapper{})
}

func TestCoursesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})

	dao.Cache = *cache

	t.Run("GET/api/courses/live", func(t *testing.T) {
		url := "/api/courses/live"

		streams := []model.Stream{
			testutils.StreamGBSLive,
			testutils.SelfStream,
			testutils.StreamFPVLive,
			testutils.StreamTensNetLive,
		}

		gbs := testutils.CourseGBS
		gbs.Visibility = "hidden"
		fpv := testutils.CourseFPV
		fpv.Visibility = "loggedin"
		tensNet := testutils.CourseTensNet

		type CourseStream struct {
			Course      model.CourseDTO
			Stream      model.StreamDTO
			LectureHall *model.LectureHallDTO
			Viewers     uint
		}

		wrapper := dao.DaoWrapper{
			StreamsDao: func() dao.StreamsDao {
				mock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
				mock.
					EXPECT().
					GetCurrentLive(gomock.Any()).
					Return(streams, nil).
					AnyTimes()
				return mock
			}(),
			CoursesDao: func() dao.CoursesDao {
				mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
				mock.
					EXPECT().
					GetCourseById(gomock.Any(), gbs.ID).
					Return(gbs, nil).
					AnyTimes()
				mock.
					EXPECT().
					GetCourseById(gomock.Any(), fpv.ID).
					Return(fpv, nil).
					AnyTimes()
				mock.
					EXPECT().
					GetCourseById(gomock.Any(), tensNet.ID).
					Return(tensNet, nil).
					AnyTimes()
				return mock
			}(),
			LectureHallsDao: func() dao.LectureHallsDao {
				mock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
				mock.
					EXPECT().
					GetLectureHallByID(gomock.Any()).
					Return(testutils.LectureHall, nil).
					AnyTimes()
				return mock
			}(),
		}

		gomino.TestCases{
			"error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							mock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetCurrentLive(gomock.Any()).
								Return([]model.Stream{}, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusNotFound,
			},
			"success not loggedin": {
				Router: func(r *gin.Engine) {
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: make([]CourseStream, 0),
			},
			"success loggedin": {
				Router: func(r *gin.Engine) {
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: []CourseStream{
					{
						Course:  fpv.ToDTO(),
						Stream:  testutils.SelfStream.ToDTO(),
						Viewers: 0,
					},
					{
						Course:      fpv.ToDTO(),
						Stream:      testutils.StreamFPVLive.ToDTO(),
						LectureHall: testutils.LectureHall.ToDTO(),
						Viewers:     0,
					},
				},
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("GET/api/courses/public", func(t *testing.T) {
		url := "/api/courses/public"

		gomino.TestCases{
			"invalid year": {
				Router:       CourseRouterWrapper,
				Url:          fmt.Sprintf("%s?year=XX&term=S", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"dao error logged-in": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetPublicAndLoggedInCourses(2023, "S").
								Return([]model.Course{}, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.CourseDTO{},
			},
			"dao error not logged-in": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetPublicCourses(2023, "S").
								Return([]model.Course{}, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.CourseDTO{},
			},
			"success logged-in": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetPublicAndLoggedInCourses(2023, "S").
								Return([]model.Course{testutils.CourseGBS, testutils.CourseFPV}, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: []model.CourseDTO{
					testutils.CourseFPV.ToDTO(),
					testutils.CourseGBS.ToDTO(),
				},
			},
			"success not logged-in": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetPublicCourses(2023, "S").
								Return([]model.Course{testutils.CourseGBS, testutils.CourseFPV}, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: []model.CourseDTO{
					testutils.CourseFPV.ToDTO(),
					testutils.CourseGBS.ToDTO(),
				},
			},
		}.
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s?year=2023&term=S", url)).
			Run(t, testutils.Equal)
	})

	t.Run("GET/api/courses/users", func(t *testing.T) {
		url := "/api/courses/users"

		gomino.TestCases{
			"invalid year": {
				Router:       CourseRouterWrapper,
				Url:          fmt.Sprintf("%s?year=XX&term=S", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"success student": {
				Router:           CourseRouterWrapper,
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.CourseDTO{},
			},
			"success lecturer": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetAdministeredCoursesByUserId(gomock.Any(), testutils.TUMLiveContextLecturer.User.ID, "S", 2023).
								Return([]model.Course{testutils.CourseGBS}, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: []model.CourseDTO{
					testutils.CourseGBS.ToDTO(),
				},
			},
			"success admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetAllCoursesForSemester(2023, "S", gomock.Any()).
								Return([]model.Course{testutils.CourseGBS, testutils.CourseFPV}).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: []model.CourseDTO{
					testutils.CourseFPV.ToDTO(),
					testutils.CourseGBS.ToDTO(),
				},
			},
		}.
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s?year=2023&term=S", url)).
			Run(t, testutils.Equal)
	})

	t.Run("GET/api/courses/users/pinned", func(t *testing.T) {
		url := "/api/courses/users/pinned"

		gomino.TestCases{
			"invalid year": {
				Router:       CourseRouterWrapper,
				Url:          fmt.Sprintf("%s?year=XX&term=S", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"not logged-in": {
				Router:           CourseRouterWrapper,
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.CourseDTO{},
			},
			"logged-in": {
				Router:           CourseRouterWrapper,
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: []model.CourseDTO{testutils.CourseFPV.ToDTO()},
			},
		}.
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s?year=2022&term=W", url)).
			Run(t, testutils.Equal)
	})

	t.Run("GET/api/courses/:slug/", func(t *testing.T) {
		url := fmt.Sprintf("/api/courses/%s/", testutils.CourseTensNet.Slug)

		response := testutils.CourseTensNet.ToDTO()
		response.Streams = []model.StreamDTO{
			testutils.StreamTensNetLive.ToDTO(),
		}

		gomino.TestCases{
			"invalid URI": {
				Router:       CourseRouterWrapper,
				Url:          "/api/courses//",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid query": {
				Router:       CourseRouterWrapper,
				Url:          fmt.Sprintf("%s?year=XX&term=W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"dao error not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseTensNet.Slug, "S", 2023).
								Return(model.Course{}, gorm.ErrRecordNotFound).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusNotFound,
			},
			"dao error internal": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseTensNet.Slug, "S", 2023).
								Return(model.Course{}, errors.New("")).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							mock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseTensNet.Slug, "S", 2023).
								Return(testutils.CourseTensNet, nil).
								AnyTimes()
							return mock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.
			Method(http.MethodGet).
			Url(fmt.Sprintf("%s?year=2023&term=S", url)).
			Run(t, testutils.Equal)
	})

	t.Run("DELETE/api/course/:courseID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/", testutils.CourseFPV.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Method: http.MethodDelete,
				Url:    url,
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								DeleteCourse(gomock.Any())
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			},
		}.Method(http.MethodDelete).Url(url).Run(t, testutils.Equal)
	})

	t.Run("DELETE/api/course/by-token/:courseID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/by-token/%d", testutils.CourseFPV.ID)
		token := "t0k3n"
		gomino.TestCases{
			"no token": {
				Router:       CourseRouterWrapper,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"course dao error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseByToken(token).
								Return(testutils.CourseFPV, errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao:   testutils.GetAuditMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodDelete).
			Url(fmt.Sprintf("%s?token=%s", url, token)).
			Run(t, testutils.Equal)
	})

	t.Run("POST/api/createCourse", func(t *testing.T) {
		url := "/api/createCourse"

		templateExecutor := tools.ReleaseTemplateExecutor{
			Template: template.Must(template.New("base").Funcs(sprig.FuncMap()).
				ParseFiles("../web/template/error.gohtml")),
		}
		tools.SetTemplateExecutor(templateExecutor)

		request := createCourseRequest{
			Access:       "enrolled",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 2020",
		}

		requestInvalidAccess := createCourseRequest{
			Access:       "abc",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 2020",
		}

		requestInvalidTerm := createCourseRequest{
			Access:       "enrolled",
			EnChat:       false,
			EnDL:         false,
			EnVOD:        false,
			Name:         "New Course",
			Slug:         "NC",
			TeachingTerm: "Sommersemester 20",
		}

		newCourse := model.Course{
			UserID:              testutils.Lecturer.ID,
			Name:                request.Name,
			Slug:                request.Slug,
			Year:                2020, // Taken from 'request'
			TeachingTerm:        "S",  // Taken from 'request'
			TUMOnlineIdentifier: request.CourseID,
			VODEnabled:          request.EnVOD,
			DownloadsEnabled:    request.EnDL,
			ChatEnabled:         request.EnChat,
			Visibility:          request.Access,
			Streams:             []model.Stream{},
			Admins:              []model.User{testutils.Lecturer},
		}

		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not lecturer": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid access": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         requestInvalidAccess,
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid term": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         requestInvalidTerm,
				ExpectedCode: http.StatusBadRequest,
			},
			"conflict with existing course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(model.Course{}, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusConflict,
			},
			"can not create course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(model.Course{}, errors.New("")).
								AnyTimes()
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), &newCourse, true).
								Return(errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not get new course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							first := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(model.Course{}, errors.New("")).Times(1)
							second := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(newCourse, errors.New("")).Times(1)

							gomock.InOrder(first, second)

							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), &newCourse, true).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturer)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			/*
				TODO: Mock tum package functions
				"success S": {
					Method:         http.MethodPost,
					Url:            url,
					TumLiveContext: &testutils.TUMLiveContextLecturer,
					DaoWrapper: dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							first := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(model.Course{}, errors.New("")).Times(1)
							second := coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), request.Slug, "S", 2020).
								Return(newCourse, nil).Times(1)

							gomock.InOrder(first, second)

							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), &newCourse, true).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
					},
					Body: bytes.NewBuffer(
						testutils.First(json.Marshal(request)).([]byte)),
					ExpectedCode: http.StatusOK,
				},
			*/
		}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})

	t.Run("POST/api/course/:courseID/copy", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/copy", testutils.CourseFPV.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"empty body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid year": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Body:         copyCourseRequest{Year: "XYZ"},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid semester": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Body:         copyCourseRequest{Year: "2023", Semester: "XY"},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"course dao error": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), true).
								Return(errors.New("")).
								AnyTimes()
							coursesMock.
								EXPECT().GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{testutils.Admin}, nil).
								MinTimes(1).MaxTimes(1)
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Body:         copyCourseRequest{Year: "2023", Semester: "Sommersemester"},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), true).
								Return(nil).
								AnyTimes()
							coursesMock.
								EXPECT().GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{testutils.Admin}, nil).
								MinTimes(1).MaxTimes(1)
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
						StreamsDao: testutils.GetStreamMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Body:         copyCourseRequest{Year: "2023", Semester: "Sommersemester"},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			},
		}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
}

func TestCoursesLectureActions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/createLecture", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/createLecture", testutils.CourseFPV.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"lectureHallId set on 'premiere'": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: createLectureRequest{
					LectureHallId: "1",
					Premiere:      true,
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid lectureHallId": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "abc",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries:    []time.Time{},
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"can not update course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								UpdateCourse(gomock.Any(), gomock.Any()).
								Return(errors.New(""))
							return coursesMock
						}(),
						AuditDao: func() dao.AuditDao {
							auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
							auditMock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
							return auditMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "1",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries: []time.Time{
						time.Now(),
					},
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(), testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								UpdateCourse(gomock.Any(), gomock.Any()).
								Return(nil)
							return coursesMock
						}(),
						AuditDao: func() dao.AuditDao {
							auditMock := mock_dao.NewMockAuditDao(gomock.NewController(t))
							auditMock.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
							return auditMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: createLectureRequest{
					Title:         "Lecture 1",
					LectureHallId: "1",
					Start:         time.Now(),
					Duration:      90,
					Premiere:      false,
					Vodup:         false,
					DateSeries: []time.Time{
						time.Now(),
					},
				},
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("POST/api/course/:courseID/deleteLecture", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/deleteLectures", testutils.CourseFPV.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid stream id in body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamGBSLive.ID)).
								Return(testutils.StreamGBSLive, nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: deleteLecturesRequest{StreamIDs: []string{
					fmt.Sprintf("%d", testutils.StreamGBSLive.ID)},
				},
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								DeleteStream(fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return().
								AnyTimes()
							return streamsMock
						}(),
						AuditDao: testutils.GetAuditMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: deleteLecturesRequest{StreamIDs: []string{
					fmt.Sprintf("%d", testutils.StreamFPVLive.ID)},
				},
				ExpectedCode: http.StatusOK,
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
	t.Run("POST/api/course/:courseID/renameLecture/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/renameLecture/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid streamID": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("/api/course/%d/renameLecture/abc", testutils.CourseFPV.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"stream not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(model.Stream{}, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateStream(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateStream(gomock.Any()).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: renameLectureRequest{
					Name: "Proofs #1",
				},
				ExpectedCode: http.StatusOK,
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
	t.Run("POST/api/course/:courseID/updateLectureSeries/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/updateLectureSeries/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"stream not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"can not update lecture series": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateLectureSeries(testutils.StreamFPVLive).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateLectureSeries(testutils.StreamFPVLive).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("DELETE/api/course/:courseID/deleteLectureSeries/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/deleteLectureSeries/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"stream not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"invalid series-identifier": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), gomock.Any()).
								Return(testutils.StreamGBSLive, nil). //StreamGBSLive.SeriesIdentifier == ""
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not delete lecture-series": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						AuditDao:   testutils.GetAuditMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), gomock.Any()).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								DeleteLectureSeries(testutils.StreamFPVLive.SeriesIdentifier).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						AuditDao:   testutils.GetAuditMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), gomock.Any()).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								DeleteLectureSeries(testutils.StreamFPVLive.SeriesIdentifier).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("PUT/api/course/:courseID/updateDescription/:streamID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/updateDescription/%d", testutils.CourseFPV.ID, testutils.StreamFPVLive.ID)

		body := renameLectureRequest{
			Name: "New lecture name!",
		}
		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid streamID": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("/api/course/%d/updateDescription/abc", testutils.CourseFPV.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateStream(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: testutils.GetStreamMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPut).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestUnits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/addUnit", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/addUnit", testutils.CourseFPV.ID)

		request := addUnitRequest{
			LectureID:   testutils.StreamFPVLive.ID,
			From:        0,
			To:          42,
			Title:       "New Unit",
			Description: "This is a new one!",
		}

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream associations": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UpdateStreamFullAssoc(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: testutils.GetStreamMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusOK,
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
	t.Run("POST/api/course/:courseID/deleteUnit/:unitID", func(t *testing.T) {
		unit := testutils.StreamFPVLive.Units[0]
		url := fmt.Sprintf("/api/course/%d/deleteUnit/%d",
			testutils.CourseFPV.ID, unit.ID)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"can not find unit": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetUnitByID(fmt.Sprintf("%d", unit.ID)).
								Return(unit, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: testutils.GetStreamMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
}

func TestCuts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/submitCut", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/submitCut", testutils.CourseFPV.ID)

		request := submitCutRequest{
			LectureID: testutils.StreamFPVLive.ID,
			From:      0,
			To:        1000,
		}
		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not update stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								SaveStream(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: testutils.GetStreamMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusOK,
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
}

func TestAdminFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/course/:courseID/admins", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins", testutils.CourseFPV.ID)

		response := []userForLecturerDto{
			{
				ID:    testutils.Admin.ID,
				Name:  testutils.Admin.Name,
				Login: testutils.Admin.GetLoginString(),
			},
			{
				ID:    testutils.Admin.ID,
				Name:  testutils.Admin.Name,
				Login: testutils.Admin.GetLoginString(),
			},
		}
		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"can not get course admins": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm,
									testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{}, errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			}}.Method(http.MethodGet).Url(url).Run(t, testutils.Equal)
	})
	t.Run("PUT/api/course/:courseID/admins/:userID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Admin.ID)
		urlStudent := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Student.ID)

		resAdmin := userForLecturerDto{
			ID:    testutils.Admin.ID,
			Name:  testutils.Admin.Name,
			Login: testutils.Admin.GetLoginString(),
		}

		resStudent := userForLecturerDto{
			ID:    testutils.Student.ID,
			Name:  testutils.Student.Name,
			Login: testutils.Student.GetLoginString(),
		}

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Method:       http.MethodPut,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid userID": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("/api/course/%d/admins/abc", testutils.CourseFPV.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"user not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.
								EXPECT().
								GetUserByID(gomock.Any(), testutils.Admin.ID).
								Return(testutils.Admin, errors.New("")).AnyTimes()
							return usersMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"can not add admin to course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(testutils.Admin.ID, testutils.CourseFPV.ID).
								Return(errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
						UsersDao: testutils.GetUsersMock(t),
						AuditDao: testutils.GetAuditMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not update user": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(testutils.Student.ID, testutils.CourseFPV.ID).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.
								EXPECT().
								GetUserByID(gomock.Any(), testutils.Student.ID).
								Return(testutils.Student, nil).
								AnyTimes()
							usersMock.
								EXPECT().
								UpdateUser(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return usersMock
						}(),
						AuditDao: testutils.GetAuditMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          urlStudent,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						UsersDao:   testutils.GetUsersMock(t),
						AuditDao:   testutils.GetAuditMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: resAdmin,
			},
			"success, user not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(testutils.Student.ID, testutils.CourseFPV.ID).
								Return(nil).
								AnyTimes()
							return coursesMock
						}(),
						UsersDao: func() dao.UsersDao {
							usersMock := mock_dao.NewMockUsersDao(gomock.NewController(t))
							usersMock.
								EXPECT().
								GetUserByID(gomock.Any(), testutils.Student.ID).
								Return(testutils.Student, nil).
								AnyTimes()
							usersMock.
								EXPECT().
								UpdateUser(gomock.Any()).
								Return(nil).
								AnyTimes()
							return usersMock
						}(),
						AuditDao: testutils.GetAuditMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:              urlStudent,
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: resStudent,
			}}.
			Method(http.MethodPut).
			Url(url).
			Run(t, testutils.Equal)
	})
	t.Run("DELETE/api/course/:courseID/admins/:userID", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/admins/%d", testutils.CourseFPV.ID, testutils.Admin.ID)

		response := userForLecturerDto{
			ID:    testutils.Admin.ID,
			Name:  testutils.Admin.Name,
			Login: testutils.Admin.GetLoginString(),
		}

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid userID": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          fmt.Sprintf("/api/course/%d/admins/abc", testutils.CourseFPV.ID),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get course admins": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{}, errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"remove last admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{testutils.Admin}, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid delete request": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{testutils.Student}, nil). // student.id != admin.id from url
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not remove admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseBySlugYearAndTerm(gomock.Any(),
									testutils.CourseFPV.Slug, testutils.CourseFPV.TeachingTerm, testutils.CourseFPV.Year).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								GetCourseAdmins(testutils.CourseFPV.ID).
								Return([]model.User{testutils.Admin, testutils.Admin}, nil).
								AnyTimes()
							coursesMock.
								EXPECT().
								RemoveAdminFromCourse(testutils.Admin.ID, testutils.CourseFPV.ID).
								Return(errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao:   testutils.GetAuditMock(t),
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			}}.
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestLectureHallsById(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/lecture-halls-by-id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lecture-halls-by-id?id=%d", testutils.CourseFPV.ID)

		ctrl := gomock.NewController(t)

		var sourceMode model.SourceMode = 0
		response := []lhResp{
			{LectureHallName: testutils.LectureHall.Name,
				LectureHallID: testutils.LectureHall.ID,
				Presets:       testutils.LectureHall.CameraPresets,
				SourceMode:    sourceMode},
		}

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid id": {
				Router:       CourseRouterWrapper,
				Method:       http.MethodGet,
				Url:          "/api/lecture-halls-by-id?id=abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusBadRequest,
			},
			"course not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, errors.New("")).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusNotFound,
			},
			"is not admin of course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(testutils.LectureHall.ID).
								Return(testutils.LectureHall, nil)
							return lectureHallMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
			},
		}.Method(http.MethodGet).Url(url).Run(t, testutils.Equal)
	})
}

func TestActivateToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/activate/:token", func(t *testing.T) {
		token := uuid.NewUUID()
		url := fmt.Sprintf("/api/course/activate/%s", token)

		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"course not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseByToken(token).
								Return(model.Course{}, errors.New(""))
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not un-delete course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseByToken(token).
								Return(model.Course{}, nil)
							coursesMock.
								EXPECT().
								UnDeleteCourse(gomock.Any(), gomock.Any()).
								Return(errors.New(""))
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							courseCopy := testutils.CourseFPV
							courseCopy.DeletedAt = gorm.DeletedAt{Valid: false}
							courseCopy.VODEnabled = true
							courseCopy.Visibility = "loggedin"

							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.
								EXPECT().
								GetCourseByToken(token).
								Return(courseCopy, nil)
							coursesMock.
								EXPECT().
								UnDeleteCourse(gomock.Any(), courseCopy).
								Return(nil)
							return coursesMock
						}(),
						AuditDao: func() dao.AuditDao {
							auditDao := mock_dao.NewMockAuditDao(ctrl)
							auditDao.EXPECT().Create(gomock.Any()).Return(nil)
							return auditDao
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestPresets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/presets", func(t *testing.T) {
		url := fmt.Sprintf("/api/course/%d/presets", testutils.CourseFPV.ID)

		var sourceMode model.SourceMode = 1
		selectedPreset := 1
		request := []lhResp{
			{
				LectureHallName: "HS-4",
				LectureHallID:   testutils.LectureHall.ID,
				Presets: []model.CameraPreset{
					{
						Name:          "Preset 1",
						PresetID:      1,
						Image:         "375ed239-c37d-450e-9d4f-1fbdb5a2dec5.jpg",
						LectureHallID: testutils.LectureHall.ID,
						IsDefault:     false,
					},
				},
				SourceMode:       sourceMode,
				SelectedPresetID: selectedPreset,
			},
		}

		presetSettings := []model.CameraPresetPreference{
			{
				LectureHallID: testutils.LectureHall.ID,
				PresetID:      selectedPreset,
			},
		}

		sourceSettings := []model.SourcePreference{
			{
				LectureHallID: testutils.LectureHall.ID,
				SourceMode:    sourceMode,
			},
		}

		afterChanges := testutils.CourseFPV
		afterChanges.SetCameraPresetPreference(presetSettings)
		afterChanges.SetSourcePreference(sourceSettings)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not update course": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()

							coursesMock.
								EXPECT().
								UpdateCourse(gomock.Any(), afterChanges).
								Return(errors.New(""))
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						AuditDao: testutils.GetAuditMock(t),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testutils.CourseFPV.ID).
								Return(testutils.CourseFPV, nil).
								AnyTimes()

							coursesMock.
								EXPECT().
								UpdateCourse(gomock.Any(), afterChanges).
								Return(nil)
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestCreateVOD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/createVOD", func(t *testing.T) {
		baseUrl := fmt.Sprintf("/api/course/%d/createVOD", testutils.CourseFPV.ID)
		url := fmt.Sprintf("%s?start=2022-07-04T10:00:00.000Z&title=VOD1", baseUrl)

		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid query": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not create stream": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(ctrl)
							streamsMock.
								EXPECT().
								CreateStream(gomock.Any()).
								Return(errors.New(""))
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
		}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestUploadVODMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/course/:courseID/uploadVODMedia", func(t *testing.T) {
		baseUrl := fmt.Sprintf("/api/course/%d/uploadVODMedia", testutils.CourseFPV.ID)
		url := fmt.Sprintf("%s?videoType=COMB&streamID=%d", baseUrl, testutils.CourseFPV.Streams[0].ID)
		urlInvalid := fmt.Sprintf("%s?videoType=XYZ&streamID=%d", baseUrl, testutils.CourseFPV.Streams[0].ID)

		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"no context": {
				Router:       CourseRouterWrapper,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"not admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			},
			"invalid query": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          baseUrl,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid video type": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          urlInvalid,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not create upload key": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: testutils.GetCoursesMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(ctrl)
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), gomock.Any()).
								Return(testutils.CourseFPV.Streams[0], nil)
							return streamsMock
						}(),
						UploadKeyDao: func() dao.UploadKeyDao {
							streamsMock := mock_dao.NewMockUploadKeyDao(ctrl)
							streamsMock.
								EXPECT().
								CreateUploadKey(gomock.Any(), gomock.Any(), gomock.Any()).
								Return(errors.New(""))
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"no workers available": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao:   testutils.GetCoursesMock(t),
						StreamsDao:   testutils.GetStreamMock(t),
						UploadKeyDao: testutils.GetUploadKeyMock(t),
						WorkerDao: func() dao.WorkerDao {
							streamsMock := mock_dao.NewMockWorkerDao(ctrl)
							streamsMock.
								EXPECT().
								GetAliveWorkers().
								Return([]model.Worker{})
							return streamsMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			/*
				TODO: Prevent p.ServeHTTP
				"success": {
					Method:         http.MethodPost,
					Url:            url,
					TumLiveContext: &testutils.TUMLiveContextAdmin,
					DaoWrapper: dao.DaoWrapper{
						CoursesDao:   testutils.GetCoursesMock(t),
						StreamsDao:   testutils.GetStreamMock(t),
						UploadKeyDao: testutils.GetUploadKeyMock(t),
						WorkerDao: func() dao.WorkerDao {
							streamsMock := mock_dao.NewMockWorkerDao(ctrl)
							streamsMock.
								EXPECT().
								GetAliveWorkers().
								Return([]model.Worker{testutils.Worker1})
							return streamsMock
						}(),
					},
					ExpectedCode: http.StatusOK,
				},
			*/
		}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestGetTranscodingProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Run("GET /api/course/:id/stream/:id/transcodingProgress", func(t *testing.T) {
		gomino.TestCases{
			"Admin, OK": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							smock := mock_dao.NewMockStreamsDao(ctrl)
							smock.EXPECT().GetStreamByID(gomock.Any(), "1969").MinTimes(1).MaxTimes(1).Return(testutils.StreamFPVNotLive, nil)
							smock.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(1969)).MinTimes(1).MaxTimes(1).Return(model.TranscodingProgress{Progress: 69}, nil)
							return smock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(40)).MinTimes(1).MaxTimes(1).Return(testutils.CourseFPV, nil)
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: "69",
			},
			"Student, Forbidden": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(ctrl)
							coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(40)).MinTimes(1).MaxTimes(1).Return(testutils.CourseFPV, nil)
							return coursesMock
						}(),
					}
					configGinCourseRouter(r, wrapper)
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedCode: http.StatusForbidden,
			}}.
			Method(http.MethodGet).
			Url("/api/course/40/stream/1969/transcodingProgress").
			Run(t, testutils.Equal)
	})
}
