package api

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/mock_dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/testutils"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSexy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const ENDPOINT_URL = "/api/sexy"

	t.Run("/api/sexy", func(t *testing.T) {
		now := time.Now()
		courses := []model.Course{
			{
				Name:       "FPV",
				Visibility: "public",
				Streams: []model.Stream{
					{
						Name:            "First lecture",
						Start:           now,
						End:             now,
						PlaylistUrl:     "/",
						PlaylistUrlPRES: "/pres",
						PlaylistUrlCAM:  "/cam",
						LiveNow:         true,
						Recording:       false,
					},
					{
						Name:            "Second lecture",
						Start:           now,
						End:             now,
						PlaylistUrl:     "/",
						PlaylistUrlPRES: "/pres",
						PlaylistUrlCAM:  "/cam",
						LiveNow:         false,
						Recording:       false,
					},
				},
			},
			{
				Name:       "GBS",
				Visibility: "hidden",
				Streams: []model.Stream{
					{
						Name:            "First lecture",
						Start:           now,
						End:             now,
						PlaylistUrl:     "/",
						PlaylistUrlPRES: "/pres",
						PlaylistUrlCAM:  "/cam",
						LiveNow:         false,
						Recording:       false,
					},
					{
						Name:            "Second lecture",
						Start:           now,
						End:             now,
						PlaylistUrl:     "/",
						PlaylistUrlPRES: "/pres",
						PlaylistUrlCAM:  "/cam",
						LiveNow:         true,
						Recording:       false,
					},
				},
			},
		}

		response := testutils.First(json.Marshal([]course{
			{
				CourseName: "FPV",
				Streams: []stream{
					{
						StreamName: "First lecture",
						Start:      now,
						End:        now,
						Sources:    []string{"/", "/pres", "/cam"},
						Live:       true,
					},
				},
			},
		})).([]byte)

		testCases := testutils.TestCases{
			"GET[GetAllCourses returns error]": testutils.TestCase{
				Method: http.MethodGet,
				Url:    ENDPOINT_URL,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.EXPECT().GetAllCourses().Return([]model.Course{}, errors.New("")).AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"GET[success]": testutils.TestCase{
				Method: http.MethodGet,
				Url:    ENDPOINT_URL,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.EXPECT().GetAllCourses().Return(courses, nil).AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: response,
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

				configGinSexyApiRouter(r, testCase.DaoWrapper)

				c.Request, _ = http.NewRequest(testCase.Method, testCase.Url, testCase.Body)
				r.ServeHTTP(w, c.Request)

				assert.Equal(t, testCase.ExpectedCode, w.Code)

				if len(testCase.ExpectedResponse) > 0 {
					assert.Equal(t, string(testCase.ExpectedResponse), w.Body.String())
				}
			})
		}
	})
}
