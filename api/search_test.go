package api

import (
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/mock_tools"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
	"net/http"
	"testing"
)

func SearchRouterWrapper(r *gin.Engine) {
	configGinSearchRouter(r, dao.DaoWrapper{}, tools.NewMeiliSearchFunctions())
}

func TestSearchFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	courseMock := getCoursesMock(t)
	streamMock := getStreamMock(t)
	meiliSearchMock := getMeiliSearchMock(t)

	t.Run("/GET/api/search", func(t *testing.T) {
		url := "/api/search"

		gomino.TestCases{
			"test": {
				Router: func(r *gin.Engine) {
					wrapper := dao.DaoWrapper{CoursesDao: courseMock, StreamsDao: streamMock}
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=red&limit=23", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
			}, /*
				"invalid semesters": {
					Router:       SearchRouterWrapper,
					Url:          fmt.Sprintf("%s?semester=203W,2024W", url),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode: http.StatusBadRequest,
				},
				"invalid semester range": {
					Router:       SearchRouterWrapper,
					Url:          fmt.Sprintf("%s?firstSemester=2045R&lastSemester=2046W", url),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode: http.StatusBadRequest,
				},
				"too many courses": {
					Router:       SearchRouterWrapper,
					Url:          fmt.Sprintf("%s?course=sdf2024W,abc2024W,def2024W", url),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode: http.StatusBadRequest,
				},
				"no user course search": {
					Router: func(r *gin.Engine) {
						wrapper := dao.DaoWrapper{
							CoursesDao: courseMock,
							StreamsDao: streamMock,
						}
						configGinSearchRouter(r, wrapper)
					},
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": []SearchCourseDTO{ToSearchCourseDTO(testutils.PublicCourse)}},
				},*/
		}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func getCoursesMock(t *testing.T) *mock_dao.MockCoursesDao {
	mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	for _, c := range testutils.AllCoursesForSearchTests {
		mock.EXPECT().GetCourseById(gomock.Any(), c.ID).Return(c, nil).AnyTimes()
		mock.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), c.Slug, c.TeachingTerm, c.Year).Return(c, nil).AnyTimes()
	}
	return mock
}

func getStreamMock(t *testing.T) *mock_dao.MockStreamsDao {
	mock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	for _, s := range testutils.AllStreamsForSearchTests {
		mock.EXPECT().GetStreamByID(gomock.Any(), s.ID).Return(s, nil).AnyTimes()
	}
	return mock
}

func getMeiliSearchMock(t *testing.T) *mock_tools.MockMeiliSearchInterface {
	mock := mock_tools.NewMockMeiliSearchInterface(gomock.NewController(t))
	mock.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return mock
}
