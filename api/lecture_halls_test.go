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
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLectureHallsCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("/createLectureHall", func(t *testing.T) {
		t.Run("POST[Invalid Body]", func(t *testing.T) {
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

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost, "/api/createLectureHall", nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("POST[Success]", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			body, _ := json.Marshal(createLectureHallRequest{
				Name:      "LH1",
				CombIP:    "0.0.0.0",
				PresIP:    "0.0.0.0",
				CamIP:     "0.0.0.0",
				CameraIP:  "0.0.0.0",
				PwrCtrlIP: "0.0.0.0",
			})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			lectureHallMock.
				EXPECT().
				CreateLectureHall(gomock.Any()).AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost, "/api/createLectureHall", bytes.NewBuffer(body))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("/lectureHall/:id", func(t *testing.T) {
		t.Run("PUT[Invalid Body]", func(t *testing.T) {
			lectureHallId := uint(1)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			configGinLectureHallApiRouter(r, dao.DaoWrapper{})

			c.Request, _ = http.NewRequest(http.MethodPut,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("PUT[id not integer]", func(t *testing.T) {
			lectureHallId := "abc"

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			configGinLectureHallApiRouter(r, dao.DaoWrapper{})

			jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

			c.Request, _ = http.NewRequest(http.MethodPut,
				fmt.Sprintf("/api/lectureHall/%s", lectureHallId), bytes.NewBuffer(jBody))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("PUT[GetLectureHallByID returns error]", func(t *testing.T) {
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
				GetLectureHallByID(lectureHallId).
				Return(model.LectureHall{},
					errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

			c.Request, _ = http.NewRequest(http.MethodPut,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("PUT[SaveLectureHall returns error]", func(t *testing.T) {
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
				GetLectureHallByID(lectureHallId).
				Return(model.LectureHall{}, nil).
				AnyTimes()
			lectureHallMock.
				EXPECT().
				SaveLectureHall(gomock.Any()).
				Return(errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

			c.Request, _ = http.NewRequest(http.MethodPut,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("PUT[success]", func(t *testing.T) {
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
				GetLectureHallByID(lectureHallId).
				Return(model.LectureHall{}, nil)
			lectureHallMock.
				EXPECT().
				SaveLectureHall(gomock.Any()).
				Return(nil)

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			jBody, _ := json.Marshal(updateLectureHallReq{CamIp: "0.0.0.0"})

			c.Request, _ = http.NewRequest(http.MethodPut,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), bytes.NewBuffer(jBody))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("DELETE[id not parameter]", func(t *testing.T) {
			lectureHallId := "abc"

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			configGinLectureHallApiRouter(r, dao.DaoWrapper{})

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

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

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

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodDelete,
				fmt.Sprintf("/api/lectureHall/%d", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("/lectureHall/:id/defaultPreset", func(t *testing.T) {
		t.Run("POST[Invalid Body]", func(t *testing.T) {
			lectureHallId := uint(1)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})

			configGinLectureHallApiRouter(r, dao.DaoWrapper{})

			c.Request, _ = http.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/lectureHall/%d/defaultPreset", lectureHallId), nil)
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("POST[FindPreset returns error]", func(t *testing.T) {
			lectureHallId := "1"
			presetId := uint(3)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})
			body, _ := json.Marshal(struct {
				PresetID uint `json:"presetID"`
			}{presetId})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			lectureHallMock.
				EXPECT().
				FindPreset(lectureHallId, fmt.Sprintf("%d", presetId)).
				Return(model.CameraPreset{}, errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/lectureHall/%s/defaultPreset", lectureHallId), bytes.NewBuffer(body))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("POST[UnsetDefaults returns error]", func(t *testing.T) {
			lectureHallId := "1"
			presetId := uint(3)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})
			body, _ := json.Marshal(struct {
				PresetID uint `json:"presetID"`
			}{presetId})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			lectureHallMock.
				EXPECT().
				FindPreset(lectureHallId, fmt.Sprintf("%d", presetId)).
				Return(model.CameraPreset{}, nil).
				AnyTimes()
			lectureHallMock.
				EXPECT().
				UnsetDefaults(gomock.Any()).
				Return(errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/lectureHall/%s/defaultPreset", lectureHallId), bytes.NewBuffer(body))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("POST[SavePreset returns error]", func(t *testing.T) {
			lectureHallId := "1"
			presetId := uint(3)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})
			body, _ := json.Marshal(struct {
				PresetID uint `json:"presetID"`
			}{presetId})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))

			lectureHallMock.
				EXPECT().
				FindPreset(lectureHallId, fmt.Sprintf("%d", presetId)).
				Return(model.CameraPreset{}, nil).
				AnyTimes()

			lectureHallMock.
				EXPECT().
				UnsetDefaults(lectureHallId).
				Return(nil).
				AnyTimes()

			lectureHallMock.
				EXPECT().
				SavePreset(gomock.Any()).
				Return(errors.New("")).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/lectureHall/%s/defaultPreset", lectureHallId), bytes.NewBuffer(body))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})

		t.Run("POST[success]", func(t *testing.T) {
			lectureHallId := "1"
			presetId := uint(3)

			w := httptest.NewRecorder()
			c, r := gin.CreateTestContext(w)

			r.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}})
			})
			body, _ := json.Marshal(struct {
				PresetID uint `json:"presetID"`
			}{presetId})

			lectureHallMock := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))

			lectureHallMock.
				EXPECT().
				FindPreset(lectureHallId, fmt.Sprintf("%d", presetId)).
				Return(model.CameraPreset{}, nil).
				AnyTimes()

			lectureHallMock.
				EXPECT().
				UnsetDefaults(lectureHallId).
				Return(nil).
				AnyTimes()

			lectureHallMock.
				EXPECT().
				SavePreset(model.CameraPreset{IsDefault: true}).
				Return(nil).
				AnyTimes()

			configGinLectureHallApiRouter(r, dao.DaoWrapper{LectureHallsDao: lectureHallMock})

			c.Request, _ = http.NewRequest(http.MethodPost,
				fmt.Sprintf("/api/lectureHall/%s/defaultPreset", lectureHallId), bytes.NewBuffer(body))
			r.ServeHTTP(w, c.Request)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	})
}

func TestCourseImport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("/course-schedule", func(t *testing.T) {
		testCases := testutils.TestCases{
			"GET[Invalid form body]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/course-schedule?;=a", // Using a semicolon makes ParseForm() return an error
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[Invalid range]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/course-schedule?range=1 to",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[Invalid from in range]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/course-schedule?range=123 to 2022-05-23",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"GET[Invalid to in range]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/course-schedule?range=2022-05-23 to 123",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
			"Get[Invalid department]": testutils.TestCase{
				Method:     http.MethodGet,
				Url:        "/api/course-schedule?range=2022-05-23 to 2022-05-24&department=Ap",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest,
			},
		}
		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, r := gin.CreateTestContext(w)

				if testCase.TumLiveContext != nil {
					r.Use(func(c *gin.Context) {
						c.Set("TUMLiveContext", *testCase.TumLiveContext)
					})
				}

				configGinLectureHallApiRouter(r, testCase.DaoWrapper)

				c.Request, _ = http.NewRequest(testCase.Method, testCase.Url, testCase.Body)
				r.ServeHTTP(w, c.Request)

				assert.Equal(t, testCase.ExpectedCode, w.Code)
			})
		}
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
				/*Contacts: []campusonline.ContactPerson{
					{FirstName: "Bernhard", LastName: "Bauer", Email: "bb@xyz.com", MainContact: false},
					{FirstName: "Hansi", LastName: "Huber", Email: "hh@xyz.com", MainContact: true},
				},*/
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
				Body:           nil,
				ExpectedCode:   http.StatusInternalServerError},
			"POST [invalid body]": testutils.TestCase{
				Method:     http.MethodPost,
				Url:        "/api/course-schedule/2022/S",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body:         nil,
				ExpectedCode: http.StatusBadRequest},
			"POST [invalid year]": testutils.TestCase{
				Method:     http.MethodPost,
				Url:        "/api/course-schedule/ABC/S",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
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
				Method:     http.MethodPost,
				Url:        "/api/course-schedule/2022/T",
				DaoWrapper: dao.DaoWrapper{},
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
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
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
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
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
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
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
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
				TumLiveContext: &tools.TUMLiveContext{User: &model.User{
					Role: model.AdminType,
				}},
				Body: bytes.NewBuffer(testutils.First(json.Marshal(importReq{
					Courses: testData,
					OptIn:   false,
				})).([]byte)),
				ExpectedCode: http.StatusOK},
		}

		for name, testCase := range testCases {
			t.Run(name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, r := gin.CreateTestContext(w)

				if testCase.TumLiveContext != nil {
					r.Use(func(c *gin.Context) {
						c.Set("TUMLiveContext", *testCase.TumLiveContext)
					})
				}

				configGinLectureHallApiRouter(r, testCase.DaoWrapper)

				c.Request, _ = http.NewRequest(testCase.Method, testCase.Url, testCase.Body)
				r.ServeHTTP(w, c.Request)

				assert.Equal(t, testCase.ExpectedCode, w.Code)
			})
		}
	})
}
