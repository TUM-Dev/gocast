package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"html/template"
	"net/http"
	"testing"
	"time"
)

func TestLectureHallsCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST/createLectureHall", func(t *testing.T) {
		url := "/api/createLectureHall"
		ctrl := gomock.NewController(t)

		body, _ := json.Marshal(createLectureHallRequest{
			Name:      "LH1",
			CombIP:    "0.0.0.0",
			PresIP:    "0.0.0.0",
			CamIP:     "0.0.0.0",
			CameraIP:  "0.0.0.0",
			PwrCtrlIP: "0.0.0.0",
		})

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"invalid body": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
						lectureHallMock.
							EXPECT().
							DeleteLectureHall(testutils.LectureHall.ID).
							Return(errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"success": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
						lectureHallMock.
							EXPECT().
							CreateLectureHall(gomock.Any()).AnyTimes()
						return lectureHallMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           bytes.NewBuffer(body),
				ExpectedCode:   http.StatusOK,
			},
		}
		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})

	t.Run("PUT/api/lectureHall/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d", testutils.LectureHall.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"invalid body": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid id": {
				Method:         http.MethodPut,
				Url:            "/api/lectureHall/abc",
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
				Body:           bytes.NewBuffer(testutils.First(json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})).([]byte)),
			},
			"can not find lecture hall": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							GetLectureHallByID(testutils.LectureHall.ID).
							Return(testutils.LectureHall, errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})).([]byte)),
			},
			"can not save lecture hall": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode: http.StatusInternalServerError,
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})).([]byte)),
			},
			"success": {
				Method:         http.MethodPut,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode: http.StatusOK,
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})).([]byte)),
			},
		}
		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
		})

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

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock}, tools.NewPresetUtility(lectureHallMock))

			c.Request, _ = http.NewRequest(http.MethodDelete,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})*/
	})

	t.Run("DELETE/api/lectureHall/:id", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d", testutils.LectureHall.ID)

		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"invalid id": {
				Method:         http.MethodDelete,
				Url:            "/api/lectureHall/abc",
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not find delete lecture hall": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							DeleteLectureHall(testutils.LectureHall.ID).
							Return(errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodDelete,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							DeleteLectureHall(testutils.LectureHall.ID).
							Return(nil).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
		})
	})

	t.Run("POST/api/lectureHall/:id/defaultPreset", func(t *testing.T) {
		url := fmt.Sprintf("/api/lectureHall/%d/defaultPreset", testutils.LectureHall.ID)
		body, _ := json.Marshal(
			struct {
				PresetID uint `json:"presetID"`
			}{
				uint(testutils.CameraPreset.PresetID),
			})
		testCases := testutils.TestCases{
			"no context": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"invalid body": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not find preset": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							FindPreset(fmt.Sprintf("%d", testutils.LectureHall.ID), fmt.Sprintf("%d", testutils.CameraPreset.PresetID)).
							Return(testutils.CameraPreset, errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				Body:         bytes.NewBuffer(body),
				ExpectedCode: http.StatusNotFound,
			},
			"can not unset preset": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(body),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not save preset": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(body),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": {
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(body),
				ExpectedCode: http.StatusOK,
			},
		}
		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(gomock.NewController(t)))
		})
	})
}

func TestCourseImport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tools.Cfg.Campus.Tokens = []string{"123", "456"} // Set tokens so that access at [1] doesn't panic
	t.Run("GET/api/course-schedule", func(t *testing.T) {
		testCases := testutils.TestCases{
			"invalid form body": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/course-schedule?;=a", // Using a semicolon makes ParseForm() return an error
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid range": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/course-schedule?range=1 to",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid from in range": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/course-schedule?range=123 to 2022-05-23",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid to in range": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/course-schedule?range=2022-05-23 to 123",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
			"invalid department": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/course-schedule?range=2022-05-23 to 2022-05-24&department=Ap",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest,
			},
		}
		testCases.Run(t, func(router *gin.Engine, daoWrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(router, daoWrapper, nil)
		})
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
		testCases := testutils.TestCases{
			"POST [no context]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/course-schedule/2022/S",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError},
			"POST [invalid body]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/course-schedule/2022/S",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusBadRequest},
			"POST [invalid year]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/course-schedule/ABC/S",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: []campusonline.Course{
						{Title: "GBS", Slug: "GBS", Import: true},
						{Title: "GDB", Slug: "GDB", Import: true},
						{Title: "FPV", Slug: "FPV", Import: true},
					},
					OptIn: false,
				})).([]byte)),
				ExpectedCode: http.StatusBadRequest},
			"POST [invalid term]": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            "/api/course-schedule/2022/T",
				DaoWrapper:     dao.DaoWrapper{},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusBadRequest},
			"POST [CreateCourse returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/course-schedule/2022/S",
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusInternalServerError},
			"POST [GetLectureHallByPartialName returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/course-schedule/2022/S",
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusOK},
			"POST [AddAdminToCourse returns error]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/course-schedule/2022/S",
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusOK},
			"POST [success]": testutils.TestCase{
				Method: http.MethodPost,
				Url:    "/api/course-schedule/2022/S",
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusOK},
		}

		testCases.Run(t, func(router *gin.Engine, daoWrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(router, daoWrapper, nil)
		})
	})
}

func TestLectureHallIcal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

	t.Run("GET/api/hall/all.ics", func(t *testing.T) {
		url := "/api/hall/all.ics"
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

		testCases := testutils.TestCases{
			"no context": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"can not get streams for lecture hall": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							GetStreamsForLectureHallIcal(gomock.Any()).
							Return(nil, errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success admin": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							GetStreamsForLectureHallIcal(testutils.TUMLiveContextAdmin.User.ID).
							Return(calendarResultsAdmin, nil).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedResponse: icalAdmin.Bytes(),
				ExpectedCode:     http.StatusOK,
			},
			"success student": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextStudent,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
						lectureHallMock.
							EXPECT().
							GetStreamsForLectureHallIcal(testutils.TUMLiveContextStudent.User.ID).
							Return(calendarResultsLoggedIn, nil).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedResponse: icalLoggedIn.Bytes(),
				ExpectedCode:     http.StatusOK,
			},
		}
		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})
}

func TestLectureHallPresets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

	t.Run("GET/refreshLectureHallPresets/:lectureHallID", func(t *testing.T) {
		url := fmt.Sprintf("/api/refreshLectureHallPresets/%d", testutils.LectureHall.ID)
		testCases := testutils.TestCases{
			"invalid id": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            "/api/refreshLectureHallPresets/abc",
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusNotFound,
			},
			"lecture hall not found": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
						lectureHallMock.
							EXPECT().
							GetLectureHallByID(testutils.LectureHall.ID).
							Return(testutils.EmptyLectureHall, errors.New("")).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				ExpectedCode: http.StatusNotFound,
			},
			"success": testutils.TestCase{
				Method:         http.MethodGet,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: testutils.GetLectureHallMock(t),
				},
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})

	t.Run("/switchPreset/:lectureHallID/:presetID/:streamID", func(t *testing.T) {
		presetId := "1"
		lectureHallId := "123"

		testCourse := testutils.CourseFPV

		ctrl := gomock.NewController(t)

		url := fmt.Sprintf("/api/course/%d/switchPreset/%s/%s/%d", testCourse.ID, lectureHallId, presetId, testutils.StreamFPVLive.ID)
		testCases := testutils.TestCases{
			"POST [no context]": testutils.TestCase{
				Method:         "POST",
				Url:            url,
				TumLiveContext: nil,
				ExpectedCode:   http.StatusInternalServerError,
			},
			"POST [stream not live]": testutils.TestCase{
				Method:         "POST",
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
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
							GetCourseById(gomock.Any(), testCourse.ID).
							Return(testCourse, nil).
							AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusBadRequest,
			},
			"POST [FindPreset returns error]": testutils.TestCase{
				Method:         "POST",
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				ExpectedCode: http.StatusNotFound,
			},
		}

		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})
}

func TestLectureHallTakeSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

	t.Run("POST/takeSnapshot/:lectureHallID/:presetID", func(t *testing.T) {
		presetIdStr := fmt.Sprintf("%d", testutils.CameraPreset.PresetID)
		lectureHallIDStr := fmt.Sprintf("%d", testutils.LectureHall.ID)

		url := fmt.Sprintf("/api/takeSnapshot/%d/%d", testutils.LectureHall.ID, testutils.CameraPreset.PresetID)
		testCases := testutils.TestCases{
			"can not find preset": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
						lectureHallMock.
							EXPECT().
							FindPreset(lectureHallIDStr, presetIdStr).
							Return(model.CameraPreset{}, errors.New("")).AnyTimes()
						return lectureHallMock
					}(),
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusNotFound,
			},
			"can not find preset after TakeSnapshot": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
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
				},
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				ExpectedCode:   http.StatusNotFound,
			},
			"success": {
				Method: http.MethodPost,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					LectureHallsDao: func() dao.LectureHallsDao {
						lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
						lectureHallMock.
							EXPECT().
							FindPreset(lectureHallIDStr, presetIdStr).
							Return(testutils.CameraPreset, nil).
							AnyTimes()
						return lectureHallMock
					}(),
				},
				TumLiveContext:   &testutils.TUMLiveContextAdmin,
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: testutils.First(json.Marshal(gin.H{"path": fmt.Sprintf("/public/%s", testutils.CameraPreset.Image)})).([]byte),
			},
		}

		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})
}

func TestLectureHallSetLH(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctrl := gomock.NewController(t)

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
		testCases := testutils.TestCases{
			"invalid body": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				Body:           nil,
				ExpectedCode:   http.StatusBadRequest,
			},
			"can not get stream by id": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
					StreamsDao: func() dao.StreamsDao {
						streamsMock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
						streamsMock.
							EXPECT().
							GetStreamsByIds(request.StreamIDs).
							Return([]model.Stream{}, errors.New("")).
							AnyTimes()
						return streamsMock
					}(),
				},
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not unset lecture hall": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(unsetLectureHallRequest)).([]byte)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"can not find lecture hall": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode: http.StatusNotFound,
			},
			"can not set lecture hall": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": testutils.TestCase{
				Method:         http.MethodPost,
				Url:            url,
				TumLiveContext: &testutils.TUMLiveContextAdmin,
				DaoWrapper: dao.DaoWrapper{
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
				},
				Body:         bytes.NewBuffer(testutils.First(json.Marshal(request)).([]byte)),
				ExpectedCode: http.StatusOK,
			},
		}

		testCases.Run(t, func(engine *gin.Engine, wrapper dao.DaoWrapper) {
			configGinLectureHallApiRouter(engine, wrapper, testutils.GetPresetUtilityMock(ctrl))
		})
	})
}
