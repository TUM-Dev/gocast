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
	"net/http"
	"testing"
	"time"
)

func TestSexy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET/api/sexy", func(t *testing.T) {
		url := "/api/sexy"

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
			"can not get courses": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
				DaoWrapper: dao.DaoWrapper{
					CoursesDao: func() dao.CoursesDao {
						coursesMock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
						coursesMock.EXPECT().GetAllCourses().Return([]model.Course{}, errors.New("")).AnyTimes()
						return coursesMock
					}(),
				},
				ExpectedCode: http.StatusInternalServerError,
			},
			"success": testutils.TestCase{
				Method: http.MethodGet,
				Url:    url,
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

		testCases.Run(t, configGinSexyApiRouter)
	})
}
