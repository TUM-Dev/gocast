package api

import (
	"errors"
	"fmt"
	"github.com/Masterminds/sprig/v3"
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
)

func CourseRouterWrapper(r *gin.Engine) {
	configGinCourseRouter(r, dao.DaoWrapper{})
}

func TestCoursesCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
