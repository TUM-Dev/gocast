package api

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

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
)

func SearchRouterWrapper(r *gin.Engine) {
	configGinSearchRouter(r, dao.DaoWrapper{}, tools.NewMeiliSearchFunctions())
}

func TestSearchCoursesFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	courseMock := getCoursesMock(t)
	streamMock := getStreamMock(t)
	wrapper := dao.DaoWrapper{CoursesDao: courseMock, StreamsDao: streamMock}
	meiliSearchMock := getMeiliSearchMock(t, wrapper)

	t.Run("GET/api/search/courses", func(t *testing.T) {
		url := "/api/search/courses"

		gomino.TestCases{
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
			"no user single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.PublicCourse),
				},
			},
		}.
			Method(http.MethodGet).
			Url(url).
			Run(t, testutils.Equal)
	})
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

		gomino.TestCases{
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
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W,%s2024W,%s2024W", url, testutils.HiddenCourse.Slug, testutils.EnrolledCourse.Slug, testutils.LoggedinCourse.Slug, testutils.PublicCourse.Slug),
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
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.PublicCourse),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamPublicCourse),
				},
			},
			"studentNoCourse single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.LoggedinCourse, testutils.PublicCourse),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
				},
			},
			"studentAllCourses single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentAllCoursesSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.EnrolledCourse, testutils.LoggedinCourse, testutils.PublicCourse),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamEnrolledCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
				},
			},
			"lecturerNoCourse single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.LoggedinCourse, testutils.PublicCourse),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
				},
			},
			"lecturerAllCourses single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerAllCoursesSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.AllCoursesForSearchTests...),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...),
				},
			},
			"admin single semester search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&semester=2024W", url),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"COURSES": ToSearchCourseDTO(testutils.AllCoursesForSearchTests...),
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...),
				},
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
			"lecturerNoCourse enrolled course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.EnrolledCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
				ExpectedCode: http.StatusBadRequest,
			},
			"no user public and hidden course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W", url, testutils.HiddenCourse.Slug, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextUserNil)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"studentNoCourse public, loggedin and hidden course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W,%s2024W", url, testutils.HiddenCourse.Slug, testutils.LoggedinCourse.Slug, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"lecturerNoCourse public, loggedin and hidden course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W,%s2024W", url, testutils.HiddenCourse.Slug, testutils.LoggedinCourse.Slug, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"lecturerAllCourses public, loggedin and enrolled course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W,%s2024W", url, testutils.EnrolledCourse.Slug, testutils.LoggedinCourse.Slug, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerAllCoursesSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamEnrolledCourse, testutils.PrivateStreamEnrolledCourse, testutils.StreamLoggedinCourse,
						testutils.PrivateStreamLoggedinCourse, testutils.StreamPublicCourse, testutils.PrivateStreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamEnrolledCourse, testutils.SubtitlesPrivateStreamEnrolledCourse,
						testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesPrivateStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse,
						testutils.SubtitlesPrivateStreamPublicCourse),
				},
			},
			"admin public, loggedin and enrolled course search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, meiliSearchMock)
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W,%s2024W,%s2024W", url, testutils.EnrolledCourse.Slug, testutils.LoggedinCourse.Slug, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS": ToSearchStreamDTO(wrapper, testutils.StreamEnrolledCourse, testutils.PrivateStreamEnrolledCourse, testutils.StreamLoggedinCourse,
						testutils.PrivateStreamLoggedinCourse, testutils.StreamPublicCourse, testutils.PrivateStreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamEnrolledCourse, testutils.SubtitlesPrivateStreamEnrolledCourse,
						testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesPrivateStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse,
						testutils.SubtitlesPrivateStreamPublicCourse),
				},
			},

			"no user all courses search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, getMeiliSearchMockReturningEveryStreamAndSubtitle(t, wrapper))
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextEmpty)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"studentNoCourse all courses search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, getMeiliSearchMockReturningEveryStreamAndSubtitle(t, wrapper))
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextStudentNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"lecturerNoCourse all courses search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, getMeiliSearchMockReturningEveryStreamAndSubtitle(t, wrapper))
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextLecturerNoCourseSearch)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.StreamHiddenCourse, testutils.StreamLoggedinCourse, testutils.StreamPublicCourse),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.SubtitlesStreamHiddenCourse, testutils.SubtitlesStreamLoggedinCourse, testutils.SubtitlesStreamPublicCourse),
				},
			},
			"admin all courses search": {
				Router: func(r *gin.Engine) {
					configGinSearchRouter(r, wrapper, getMeiliSearchMockReturningEveryStreamAndSubtitle(t, wrapper))
				},
				Url:          fmt.Sprintf("%s?q=testen&course=%s2024W", url, testutils.PublicCourse.Slug),
				Middlewares:  testutils.GetMiddlewares(tools.ErrorHandler, testutils.TUMLiveContext(testutils.TUMLiveContextAdmin)),
				ExpectedCode: http.StatusOK,
				ExpectedResponse: MeiliSearchMap{
					"STREAMS":   ToSearchStreamDTO(wrapper, testutils.AllStreamsForSearchTests...),
					"SUBTITLES": ToSearchSubtitleDTO(wrapper, testutils.AllSubtitlesForSearchTests...),
				},
			},
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
			streams, _ := tools.ToMeiliStreams(testutils.AllStreamsForSearchTests, daoWrapper)
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "COURSES", Hits: meiliCourseSliceToInterfaceSlice(tools.ToMeiliCourses(testutils.AllCoursesForSearchTests))},
				{IndexUID: "STREAMS", Hits: meiliStreamSliceToInterfaceSlice(streams)},
			}}
		}).AnyTimes()

	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 4, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "COURSES", Hits: meiliCourseSliceToInterfaceSlice(tools.ToMeiliCourses(testutils.AllCoursesForSearchTests))},
			}}
		}).AnyTimes()

	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 3, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			streams := make([]model.Stream, 0)
			subtitles := make([]tools.MeiliSubtitles, 0)

			// find indexes for id arrays
			s := regexp.MustCompile(`\[`)
			c := regexp.MustCompile(`]`)
			startIndexes := s.FindAllIndex([]byte(streamFilter), -1)
			endIndexes := c.FindAllIndex([]byte(streamFilter), -1)

			for i, startIndex := range startIndexes {
				idsAsStrings := strings.Split(streamFilter[startIndex[1]:endIndexes[i][0]], ",")
				for _, idString := range idsAsStrings {
					id, _ := strconv.Atoi(idString)
					for _, stream := range testutils.AllStreamsForSearchTests {
						if stream.CourseID == uint(id) {
							streams = append(streams, stream)
						}
					}
				}
			}

			for _, subtitle := range testutils.AllSubtitlesForSearchTests {
				if slices.ContainsFunc(streams, func(stream model.Stream) bool {
					return stream.ID == subtitle.StreamID
				}) {
					subtitles = append(subtitles, subtitle)
				}
			}
			returnStreams, _ := tools.ToMeiliStreams(streams, daoWrapper)
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "STREAMS", Hits: meiliStreamSliceToInterfaceSlice(returnStreams)},
				{IndexUID: "SUBTITLES", Hits: meiliSubtitleSliceToInterfaceSlice(subtitles)},
			}}
		}).AnyTimes()
	return mock
}

func getMeiliSearchMockReturningEveryStreamAndSubtitle(t *testing.T, daoWrapper dao.DaoWrapper) *mock_tools.MockMeiliSearchInterface {
	mock := mock_tools.NewMockMeiliSearchInterface(gomock.NewController(t))
	mock.EXPECT().Search(gomock.Any(), gomock.Any(), 3, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(q interface{}, limit interface{}, searchType interface{}, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
			streams, _ := tools.ToMeiliStreams(testutils.AllStreamsForSearchTests, daoWrapper)
			return &meilisearch.MultiSearchResponse{Results: []meilisearch.SearchResponse{
				{IndexUID: "STREAMS", Hits: meiliStreamSliceToInterfaceSlice(streams)},
				{IndexUID: "SUBTITLES", Hits: meiliSubtitleSliceToInterfaceSlice(testutils.AllSubtitlesForSearchTests)},
			}}
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
