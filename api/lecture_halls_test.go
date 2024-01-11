package api

import (
	"bytes"
	"errors"
	"fmt"
	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
	"html/template"
	"net/http"
	"testing"
	"time"
)

func LectureHallRouterWrapper(t *testing.T) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		configGinLectureHallApiRouter(r, dao.DaoWrapper{}, testutils.GetPresetUtilityMock(gomock.NewController(t)))
	}
}

func TestLectureHallsCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/createLectureHall", func(t *testing.T) {
		url := "/api/createLectureHall"
		ctrl := gomock.NewController(t)

		body := createLectureHallRequest{
			Name:      "LH1",
			CombIP:    "0.0.0.0",
			PresIP:    "0.0.0.0",
			CamIP:     "0.0.0.0",
			CameraIP:  "0.0.0.0",
			PwrCtrlIP: "0.0.0.0",
		}

		gomino.TestCases{
			"no context": {
				Router:       LectureHallRouterWrapper(t),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid body": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							lectureHallMock.
								EXPECT().
								DeleteLectureHall(testutils.LectureHall.ID).
								Return(errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							lectureHallMock.
								EXPECT().
								CreateLectureHall(gomock.Any()).AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("PUT/api/lectureHall/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d", testutils.LectureHall.ID)
		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"no context": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid id": {
				Url:          "/api/lectureHall/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
				Body:         updateLectureHallReq{CamIp: "0.0.0.0"},
			},
			"can not find lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(testutils.LectureHall.ID).
								Return(testutils.LectureHall, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
				Body:         updateLectureHallReq{CamIp: "0.0.0.0"},
			},
			"can not save lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(testutils.LectureHall.ID).
								Return(testutils.LectureHall, nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								SaveLectureHall(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
				Body:         updateLectureHallReq{CamIp: "0.0.0.0"},
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(testutils.LectureHall.ID).
								Return(testutils.LectureHall, nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								SaveLectureHall(gomock.Any()).
								Return(nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				Body:         updateLectureHallReq{CamIp: "0.0.0.0"},
			}}.
			Router(LectureHallRouterWrapper(t)).
			Method(http.MethodPut).
			Url(url).
			Run(t, testutils.Equal)

		/*t.Run("DELETE[id not parameter]", func(t *testing.T) {
			lectureHallId := "abc"

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			configGinLectureHallApiRouter(r, dao.DaoWrapper{}, tools.NewPresetUtility(nil))

			c.Request, _ = http.NewRequest(http.MethodDelete,
				fmt.Sprintf("/api/lectureHall/%s", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("DELETE[DeleteLectureHall returns error]", func(t *testing.T) {
			lectureHallId := uint(1)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			lectureHallMock.
				EXPECT().
				DeleteLectureHall(lectureHallId).
				Return(errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock}, tools.NewPresetUtility(lectureHallMock))

			c.Request, _ = http.NewRequest(http.MethodDelete,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("DELETE[success]", func(t *testing.T) {
			lectureHallId := uint(1)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			lectureHallMock.
				EXPECT().
				DeleteLectureHall(lectureHallId).
				Return(nil).
				AnyTimes()
			r.Use(tools.ErrorHandler)
			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock}, tools.NewPresetUtility(lectureHallMock))

			c.Request, _ = http.NewRequest(http.MethodDelete,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})*/
	})

	t.Run("DELETE/api/lectureHall/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d", testutils.LectureHall.ID)
		ctrl := gomock.NewController(t)

		gomino.TestCases{
			"no context": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid id": {
				Url:          "/api/lectureHall/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find delete lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								DeleteLectureHall(testutils.LectureHall.ID).
								Return(errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								DeleteLectureHall(testutils.LectureHall.ID).
								Return(nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(ctrl))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Router(LectureHallRouterWrapper(t)).
			Method(http.MethodDelete).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("POST/api/lectureHall/:id/defaultPreset", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d/defaultPreset", testutils.LectureHall.ID)
		body := struct {
			PresetID uint `json:"presetID"`
		}{
			uint(testutils.CameraPreset.PresetID),
		}
		gomino.TestCases{
			"no context": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"invalid body": {
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not find preset": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								FindPreset(fmt.Sprintf("%d", testutils.LectureHall.ID), fmt.Sprintf("%d", testutils.CameraPreset.PresetID)).
								Return(testutils.CameraPreset, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusNotFound,
			},
			"can not unset preset": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								FindPreset(fmt.Sprintf("%d", testutils.LectureHall.ID), fmt.Sprintf("%d", testutils.CameraPreset.PresetID)).
								Return(testutils.CameraPreset, nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								UnsetDefaults(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not save preset": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								FindPreset(fmt.Sprintf("%d", testutils.LectureHall.ID), fmt.Sprintf("%d", testutils.CameraPreset.PresetID)).
								Return(testutils.CameraPreset, nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								UnsetDefaults(gomock.Any()).
								Return(nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								SavePreset(gomock.Any()).
								Return(errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Method:       http.MethodPost,
				Url:          url,
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								FindPreset(fmt.Sprintf("%d", testutils.LectureHall.ID), fmt.Sprintf("%d", testutils.CameraPreset.PresetID)).
								Return(testutils.CameraPreset, nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								UnsetDefaults(gomock.Any()).
								Return(nil).
								AnyTimes()
							lectureHallMock.
								EXPECT().
								SavePreset(gomock.Any()).
								Return(nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         body,
				ExpectedCode: http.StatusOK,
			}}.
			Router(LectureHallRouterWrapper(t)).
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestCourseImport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tools.Cfg.Campus.Tokens = []string{"123", "456"} // Set tokens so that access at [1] doesn't panic
	t.Run("GET/api/course-schedule", func(t *testing.T) {
		gomino.TestCases{
			"invalid form body": {
				Url:          "/api/course-schedule?;=a", // Using a semicolon makes ParseForm() return an error
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid range": {
				Url:          "/api/course-schedule?range=1 to",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid from in range": {
				Url:          "/api/course-schedule?range=123 to 2022-05-23",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid to in range": {
				Url:          "/api/course-schedule?range=2022-05-23 to 123",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"invalid department": {
				Url:          "/api/course-schedule?range=2022-05-23 to 2022-05-24&department=Ap",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			}}.
			Router(LectureHallRouterWrapper(t)).
			Method(http.MethodGet).
			Run(t, testutils.Equal)
	})

	t.Run("/course-schedule/:year/:term", func(t *testing.T) {
		// importReq taken from courseimport.go
		type importReq struct {
			Courses []campusonline.Course `json:"courses"`
			OptIn   bool                  `json:"optIn"`
		}
		testData := []campusonline.Course{
			{Title: "GBS",
				Slug:   "GBS",
				Import: false,
				Events: []campusonline.Event{{RoomName: "1"}},
			},
			{Title: "GDB",
				Slug:   "GDB",
				Import: true,
				Events: []campusonline.Event{{RoomName: "1"}},
			},
			{Title: "FPV",
				Slug:   "FPV",
				Import: true,
				Events: []campusonline.Event{{RoomName: "1"}},
			},
		}
		gomino.TestCases{
			"POST [no context]": {
				Url:          "/api/course-schedule/2022/S",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError},
			"POST [invalid body]": {
				Url:          "/api/course-schedule/2022/S",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest},
			"POST [invalid year]": {
				Url:         "/api/course-schedule/ABC/S",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: []campusonline.Course{
						{Title: "GBS", Slug: "GBS", Import: true},
						{Title: "GDB", Slug: "GDB", Import: true},
						{Title: "FPV", Slug: "FPV", Import: true},
					},
					OptIn: false,
				},
				ExpectedCode: http.StatusBadRequest},
			"POST [invalid term]": {
				Url:         "/api/course-schedule/2022/T",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: testData,
					OptIn:   false,
				},
				ExpectedCode: http.StatusBadRequest},
			"POST [CreateCourse returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByPartialName("1").
								Return(model.LectureHall{}, nil).
								AnyTimes()
							return lectureHallMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).
								Return(errors.New("error")).AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							return coursesMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Url:         "/api/course-schedule/2022/S",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: testData,
					OptIn:   false,
				},
				ExpectedCode: http.StatusInternalServerError},
			"POST [GetLectureHallByPartialName returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByPartialName("1").
								Return(model.LectureHall{}, errors.New("error")).
								AnyTimes()
							return lectureHallMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							return coursesMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Url:         "/api/course-schedule/2022/S",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: testData,
					OptIn:   false,
				},
				ExpectedCode: http.StatusOK},
			"POST [AddAdminToCourse returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByPartialName("1").
								Return(model.LectureHall{}, nil).
								AnyTimes()
							return lectureHallMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(errors.New("error")).AnyTimes()
							return coursesMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Url:         "/api/course-schedule/2022/S",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: testData,
					OptIn:   false,
				},
				ExpectedCode: http.StatusOK,
			},
			"POST [success]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByPartialName("1").
								Return(model.LectureHall{}, nil).
								AnyTimes()
							return lectureHallMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							coursesMock.
								EXPECT().
								AddAdminToCourse(gomock.Any(), gomock.Any()).
								Return(nil).AnyTimes()
							return coursesMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Url:         "/api/course-schedule/2022/S",
				Middlewares: testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body: importReq{
					Courses: testData,
					OptIn:   false,
				},
				ExpectedCode: http.StatusOK,
			}}.
			Router(LectureHallRouterWrapper(t)).
			Method(http.MethodPost).
			Run(t, testutils.Equal)
	})
}

func TestLectureHallIcal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/schedule.ics", func(t *testing.T) {
		url := "/api/schedule.ics?lecturehalls=1,2"
		calendarResultsAdmin := []dao.CalendarResult{
			{
				StreamID:        1,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "FPV",
				LectureHallName: "HS1",
			},

			{
				StreamID:        2,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "GBS",
				LectureHallName: "HS2",
			},
		}
		calendarResultsLoggedIn := []dao.CalendarResult{
			{
				StreamID:        1,
				Created:         time.Now(),
				Start:           time.Now(),
				End:             time.Now(),
				CourseName:      "FPV",
				LectureHallName: "HS1",
			},
		}
		var icalAdmin bytes.Buffer
		var icalLoggedIn bytes.Buffer
		templ, _ := template.ParseFS(staticFS, "template/*.gotemplate")
		_ = templ.ExecuteTemplate(&icalAdmin, "ical.gotemplate", calendarResultsAdmin)
		_ = templ.ExecuteTemplate(&icalLoggedIn, "ical.gotemplate", calendarResultsLoggedIn)

		gomino.TestCases{
			"no context": {
				Router:       LectureHallRouterWrapper(t),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not get streams for lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(gomock.Any(), []uint{1, 2}, false).
								Return(nil, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
						AuditDao: func() dao.AuditDao {
							auditDao := mock_dao.NewMockAuditDao(gomock.NewController(t))
							auditDao.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
							return auditDao
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success admin": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(testutils.TUMLiveContextAdmin.User.ID, []uint{1, 2}, false).
								Return(calendarResultsAdmin, nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedResponse: icalAdmin.Bytes(),
				ExpectedCode:     http.StatusOK,
			},
			"success student": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetStreamsForLectureHallIcal(testutils.TUMLiveContextStudent.User.ID, []uint{1, 2}, false).
								Return(calendarResultsLoggedIn, nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
				ExpectedResponse: icalLoggedIn.Bytes(),
				ExpectedCode:     http.StatusOK,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestLectureHallPresets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

	t.Run("GET/refreshLectureHallPresets/:lectureHallID", func(t *testing.T) {
		url := fmt.Sprintf("/api/refreshLectureHallPresets/%d", testutils.LectureHall.ID)
		gomino.TestCases{
			"invalid id": {
				Router:       LectureHallRouterWrapper(t),
				Url:          "/api/refreshLectureHallPresets/abc",
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"lecture hall not found": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(testutils.LectureHall.ID).
								Return(testutils.EmptyLectureHall, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: testutils.GetLectureHallMock(t),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})

	t.Run("/switchPreset/:lectureHallID/:presetID/:streamID", func(t *testing.T) {
		presetId := "1"
		lectureHallId := "123"

		testCourse := testutils.CourseFPV

		url := fmt.Sprintf("/api/course/%d/switchPreset/%s/%s/%d", testCourse.ID, lectureHallId, presetId, testutils.StreamFPVLive.ID)
		gomino.TestCases{
			"POST [no context]": {
				Router:       LectureHallRouterWrapper(t),
				ExpectedCode: http.StatusInternalServerError,
			},
			"POST [stream not live]": {
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
								GetCourseById(gomock.Any(), testCourse.ID).
								Return(testCourse, nil).
								AnyTimes()
							return coursesMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"POST [FindPreset returns error]": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamByID(gomock.Any(), fmt.Sprintf("%d", testutils.StreamFPVLive.ID)).
								Return(testutils.StreamFPVLive, nil).AnyTimes()
							return streamsMock
						}(),
						CoursesDao: func() dao.CoursesDao {
							coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
							coursesMock.
								EXPECT().
								GetCourseById(gomock.Any(), testCourse.ID).
								Return(testCourse, nil).
								AnyTimes()
							return coursesMock
						}(),
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								FindPreset(lectureHallId, presetId).
								Return(model.CameraPreset{}, errors.New("")).AnyTimes()
							return lectureHallMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			}}.
			Method(http.MethodPost).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func TestLectureHallTakeSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

	t.Run("POST/takeSnapshot/:lectureHallID/:presetID", func(t *testing.T) {
		presetIdStr := fmt.Sprintf("%d", testutils.CameraPreset.PresetID)
		lectureHallIDStr := fmt.Sprintf("%d", testutils.LectureHall.ID)

		url := fmt.Sprintf("/api/takeSnapshot/%d/%d", testutils.LectureHall.ID, testutils.CameraPreset.PresetID)
		gomino.TestCases{
			"can not find preset": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							lectureHallMock.
								EXPECT().
								FindPreset(lectureHallIDStr, presetIdStr).
								Return(model.CameraPreset{}, errors.New("")).AnyTimes()
							return lectureHallMock
						}(),
					}

					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"can not find preset after TakeSnapshot": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							first := lectureHallMock.
								EXPECT().
								FindPreset(lectureHallIDStr, presetIdStr).
								Return(testutils.CameraPreset, nil)
							second := lectureHallMock.
								EXPECT().
								FindPreset(lectureHallIDStr, presetIdStr).
								Return(testutils.CameraPreset, errors.New(""))
							gomock.InOrder(first, second)
							return lectureHallMock
						}(),
					}

					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusNotFound,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
							lectureHallMock.
								EXPECT().
								FindPreset(lectureHallIDStr, presetIdStr).
								Return(testutils.CameraPreset, nil).
								AnyTimes()
							return lectureHallMock
						}(),
					}

					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: gin.H{"path": fmt.Sprintf("/public/%s", testutils.CameraPreset.Image)},
			}}.Method(http.MethodPost).Url(url).Run(t, testutils.Equal)
	})
}

func TestLectureHallSetLH(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/api/setLectureHall", func(t *testing.T) {
		url := "/api/setLectureHall"
		lectureHall := testutils.LectureHall
		fpvStream := testutils.StreamFPVLive
		request := setLectureHallRequest{
			StreamIDs:     []uint{fpvStream.ID},
			LectureHallID: lectureHall.ID,
		}
		unsetLectureHallRequest := setLectureHallRequest{
			StreamIDs:     []uint{fpvStream.ID},
			LectureHallID: 0,
		}
		gomino.TestCases{
			"invalid body": {
				Router:       LectureHallRouterWrapper(t),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusBadRequest,
			},
			"can not get stream by id": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{}, errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not unset lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         unsetLectureHallRequest,
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not find lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: func() dao.LectureHallsDao {
							lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
							lectureHallMock.
								EXPECT().
								GetLectureHallByID(lectureHall.ID).
								Return(model.LectureHall{}, errors.New("")).
								AnyTimes()
							return lectureHallMock
						}(),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusNotFound,
			},
			"can not set lecture hall": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: testutils.GetLectureHallMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								SetLectureHall(request.StreamIDs, request.LectureHallID).
								Return(errors.New("")).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
				},
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				Body:         request,
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{
						LectureHallsDao: testutils.GetLectureHallMock(t),
						StreamsDao: func() dao.StreamsDao {
							streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
							streamsMock.
								EXPECT().
								GetStreamsByIds(request.StreamIDs).
								Return([]model.Stream{fpvStream}, nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								UnsetLectureHall(request.StreamIDs).
								Return(nil).
								AnyTimes()
							streamsMock.
								EXPECT().
								SetLectureHall(request.StreamIDs, request.LectureHallID).
								Return(nil).
								AnyTimes()
							return streamsMock
						}(),
					}
					configGinLectureHallApiRouter(r, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
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
