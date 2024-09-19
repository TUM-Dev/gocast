package api

import (
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/mock_tools"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/matthiasreumann/gomino"
	"github.com/meilisearch/meilisearch-go"
	"net/http"
	"strconv"
	"testing"
)

var (
	emptySlice = []any{}
)

func SearchRouterWrapper(r *gin.Engine) {
	configGinSearchRouter(r, dao.DaoWrapper{}, tools.NewMeiliSearchFunctions())
}

func TestSearchFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	courseMock := getCoursesMock(t)
	streamMock := getStreamMock(t)
	wrapper := dao.DaoWrapper{CoursesDao: courseMock, StreamsDao: streamMock}
	meiliSearchMock := getMeiliSearchMock(t, wrapper)

	// these tests ensure that even if meilisearch returns courses/streams/subtitles that the user is not allowed to see, these will not be passed on to the client
	t.Run("GET/api/search", func(t *testing.T) {
		url := "/api/search"

		gomino.TestCases{ /*
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
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:          fmt.Sprintf("%s?course=sdf2024W,abc2024W,def2024W", url),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode: http.StatusBadRequest,
				},
				"all semesters search success": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.PublicCourse)},
				},
				"no user single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.PublicCourse), "STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamPublicCourse)},
				},
				"studentNoCourse single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.LoggedinCourse, testutils.PublicCourse), "STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse)},
				},
				"studentAllCourses single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentAllCoursesSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.EnrolledCourse, testutils.LoggedinCourse, testutils.PublicCourse), "STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamEnrolledCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse)},
				},
				"lecturerNoCourse single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.LoggedinCourse, testutils.PublicCourse), "STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse)},
				},
				"lecturerAllCourses single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerAllCoursesSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.AllCoursesForSearchTests...), "STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...)},
				},
				"admin single semester search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&semester=2024W", url),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"COURSES": ToSearchCourseDTO(testutils.AllCoursesForSearchTests...), "STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...)},
				},

				"no user loggedin course search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.LoggedinCourse.Slug),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
					ExpectedCode: http.StatusBadRequest,
				},
				"studentNoCourse enrolled course search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.EnrolledCourse.Slug),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
					ExpectedCode: http.StatusBadRequest,
				},
				"lecturerNoCourse hidden course search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.HiddenCourse.Slug),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
					ExpectedCode: http.StatusBadRequest,
				},
				"student two courses search success": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W", url, testutils.PublicCourse.Slug, testutils.LoggedinCourse.Slug),
					Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudent)),
					ExpectedCode: http.StatusOK,
				},*/
			"no user public course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:              fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
				Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode:     http.StatusOK,
				ExpectedResponse: MeiliSearchMap{"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamPublicCourse), "SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamPublicCourse)},
			}, /*
				"studentNoCourse public and loggedin course search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					Url:              fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W", url, testutils.PublicCourse.Slug, testutils.LoggedinCourse.Slug),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse), "SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse)},
				},
				"lecturerAllCourses all courses search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					//meilisearchmock returns subtitles from all courses, meaning the parameters in the url dont actually match the search results in this testcase
					Url:              fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerAllCoursesSearch)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...), "SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.AllSubtitlesForSearchTests...)},
				},
				"admin all courses search": {
					Router: func(r *gin.Engine) {
						configGinSearchRouter(r, wrapper, meiliSearchMock)
					},
					//meilisearchmock returns subtitles from all courses, meaning the parameters in the url dont actually match the search results in this testcase
					Url:              fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
					Middlewares:      testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
					ExpectedCode:     http.StatusOK,
					ExpectedResponse: MeiliSearchMap{"STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...), "SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.AllSubtitlesForSearchTests...)},
				},*/
		}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
}

func getCoursesMock(t *testing.T) *mock_dao.MockCoursesDao {
	mock := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	for _, course := range testutils.AllCoursesForSearchTests {
		mock.EXPECT().GetCourseById(gomock.Any(), course.ID).Return(course, nil).AnyTimes()
		mock.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), course.Slug, course.TeachingTerm, course.Year).Return(course, nil).AnyTimes()
	}
	mock.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).Return(model.Course{}, errors.New("whoops")).AnyTimes()
	mock.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(model.Course{}, errors.New("whoops")).AnyTimes()
	return mock
}

func getStreamMock(t *testing.T) *mock_dao.MockStreamsDao {
	mock := mock_dao.NewMockStreamsDao(gomock.NewController(t))
	for _, s := range testutils.AllStreamsForSearchTests {
		mock.EXPECT().GetStreamByID(gomock.Any(), strconv.Itoa(int(s.ID))).Return(s, nil).AnyTimes()
	}
	mock.EXPECT().GetStreamByID(gomock.Any(), gomock.Any()).Return(model.Stream{}, errors.New("whoops")).AnyTimes()
	return mock
}

func getMeiliSearchMock(t *testing.T, daoWrapper dao.DaoWrapper) *mock_tools.MockMeiliSearchInterface {
	mock := mock_tools.NewMockMeiliSearchInterface(gomock.NewController(t))
	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 6, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			streams, _ := tools.ToMeiliStreams(testutils.AllStreamsForSearchTests, daoWrapper.CoursesDao)
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "COURSES", Hits: meiliCourseSliceToInterfaceSlice(tools.ToMeiliCourses(testutils.AllCoursesForSearchTests))},
				{IndexUID: "STREAMS", Hits: meiliStreamSliceToInterfaceSlice(streams)}}}
		}).AnyTimes()

	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 4, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "COURSES", Hits: meiliCourseSliceToInterfaceSlice(tools.ToMeiliCourses(testutils.AllCoursesForSearchTests))}}}
		}).AnyTimes()

	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 3, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			streams, _ := tools.ToMeiliStreams(testutils.AllStreamsForSearchTests, daoWrapper.CoursesDao)
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "STREAMS", Hits: meiliStreamSliceToInterfaceSlice(streams)},
				{IndexUID: "SUBTITLES", Hits: meiliSubtitleSliceToInterfaceSlice(testutils.AllSubtitlesForSearchTests)}}}
		}).AnyTimes()
	return mock
}

func meiliCourseSliceToInterfaceSlice(cs []tools.MeiliCourse) []interface{} {
	s := make([]interface{}, len(cs))
	for i, c := range cs {
		s[i] = c
	}
	return s
}

func meiliStreamSliceToInterfaceSlice(cs []tools.MeiliStream) []interface{} {
	s := make([]interface{}, len(cs))
	for i, c := range cs {
		s[i] = c
	}
	return s
}

func meiliSubtitleSliceToInterfaceSlice(cs []tools.MeiliSubtitles) []interface{} {
	s := make([]interface{}, len(cs))
	for i, c := range cs {
		s[i] = c
	}
	return s
}
