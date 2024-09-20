package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"github.com/meilisearch/meilisearch-go"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper, meiliSearchInstance tools.MeiliSearchInterface) {
	routes := searchRoutes{daoWrapper, meiliSearchInstance}

	searchGroup := router.Group("/api/search")
	searchGroup.GET("", routes.search)
	withStream := searchGroup.Group("/stream/:streamID")
	withStream.Use(tools.InitStream(daoWrapper))
	withStream.GET("/subtitles", routes.searchSubtitles)

	/*withCourse := searchGroup.Group("/course/:courseID")
	withCourse.Use(tools.InitCourse(daoWrapper))
	//withCourse.GET("/streams", routes.searchStreams)*/

	searchGroup.GET("/courses", routes.searchCourses)
}

type searchRoutes struct {
	dao.DaoWrapper
	m tools.MeiliSearchInterface
}

const (
	FilterMaxCoursesCount = 3
	DefaultLimit          = 10
)

type MeiliSearchMap map[string]any

func (r searchRoutes) searchSubtitles(c *gin.Context) {
	s := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).Stream
	q := c.Query("q")
	c.JSON(http.StatusOK, r.m.SearchSubtitles(q, s.ID))
}

/*
für alle:
q=...&limit=...

Format für Semester:2024W
Format für Kurs:<Slug><Semester>
Einzelnes Semester:
semester=...
firstSemester=1234X&lastSemester=1234X

Mehrere Semester:
firstSemester=...&lastSemester=...
semester=...,..., max. 8

Einzelner oder Mehrere Kurse:
course=...
course=...,... max. 2
*/

func (r searchRoutes) searchCourses(c *gin.Context) {
	user, query, limit := getDefaultParameters(c)

	res, err := semesterSearchHelper(c, r.m, query, int64(limit), user, true)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if res == nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	checkAndFillResponse(c, user, int64(limit), r.DaoWrapper, res, false)
	c.JSON(http.StatusOK, responseToMap(res))
}

func getDefaultParameters(c *gin.Context) (*model.User, string, uint64) {
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	query := c.Query("q")
	limit, err := strconv.ParseUint(c.Query("limit"), 10, 16)
	if err != nil || limit > math.MaxInt64 { //second condition should never happen, max bitSize for parseuint is 16
		limit = DefaultLimit
	}
	return user, query, limit
}

func (r searchRoutes) search(c *gin.Context) {
	user, query, limit := getDefaultParameters(c)

	var res *meilisearch.MultiSearchResponse
	if courseParam := c.Query("course"); courseParam != "" {
		courses, errorCode := parseCourses(c, r.DaoWrapper, courseParam)
		if errorCode == 2 {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if errorCode != 0 || len(courses) > FilterMaxCoursesCount {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		for _, course := range courses {
			if !user.IsEligibleToWatchCourse(course) {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}
		res = r.m.Search(query, int64(limit), 3, "", meiliStreamFilter(c, user, model.Semester{}, courses), meiliSubtitleFilter(user, courses))
		if res == nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		checkAndFillResponse(c, user, int64(limit), r.DaoWrapper, res, true)
		c.JSON(http.StatusOK, responseToMap(res))
		return
	}

	res, err := semesterSearchHelper(c, r.m, query, int64(limit), user, false)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if res == nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	checkAndFillResponse(c, user, int64(limit), r.DaoWrapper, res, false)
	c.JSON(http.StatusOK, responseToMap(res))
	return
}

func responseToMap(res *meilisearch.MultiSearchResponse) MeiliSearchMap {
	msm := make(MeiliSearchMap)
	if res == nil {
		return msm
	}
	for _, r := range res.Results {
		msm[r.IndexUID] = r.Hits
	}
	return msm
}

func semesterSearchHelper(c *gin.Context, m tools.MeiliSearchInterface, query string, limit int64, user *model.User, courseSearchOnly bool) (*meilisearch.MultiSearchResponse, error) {
	var res *meilisearch.MultiSearchResponse
	firstSemesterParam := c.Query("firstSemester")
	lastSemesterParam := c.Query("lastSemester")
	semestersParam := c.Query("semester")
	if firstSemesterParam != "" && lastSemesterParam != "" || semestersParam != "" {
		var firstSemester, lastSemester model.Semester
		semesters1, err1 := parseSemesters(firstSemesterParam)
		semesters2, err2 := parseSemesters(lastSemesterParam)
		semesters, err3 := parseSemesters(semestersParam)
		if (err1 != nil || err2 != nil || len(semesters1) > 1 || len(semesters2) > 1) && err3 != nil {
			return nil, errors.New("wrong parameters")
		}
		rangeSearch := false
		if len(semesters1) > 0 && len(semesters2) > 0 {
			firstSemester = semesters1[0]
			lastSemester = semesters2[0]
			rangeSearch = true
		}

		if !courseSearchOnly && (rangeSearch && firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm || len(semesters) == 1) {
			// single semester search
			var semester model.Semester
			if rangeSearch {
				semester = firstSemester
			} else {
				semester = semesters[0]
			}
			res = m.Search(query, limit, 6, meiliCourseFilter(c, user, firstSemester, lastSemester, semesters), meiliStreamFilter(c, user, semester, nil), "")
		} else {
			// multiple semester search
			res = m.Search(query, limit, 4, meiliCourseFilter(c, user, firstSemester, lastSemester, semesters), "", "")
		}
		return res, nil
	}

	sem1 := model.Semester{TeachingTerm: "S"}
	sem2 := model.Semester{TeachingTerm: "W", Year: 3000}
	return m.Search(query, limit, 4, meiliCourseFilter(c, user, sem1, sem2, nil), "", ""), nil
}

type SearchCourseDTO struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Year         int    `json:"year"`
	TeachingTerm string `json:"semester"`
}

type SearchStreamDTO struct {
	ID           uint   `json:"ID"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CourseName   string `json:"courseName"`
	Year         int    `json:"year"`
	TeachingTerm string `json:"semester"`
	CourseSlug   string `json:"courseSlug"`
}

type SearchSubtitlesDTO struct {
	StreamID           uint      `json:"streamID"`
	Timestamp          int64     `json:"timestamp"`
	TextPrev           string    `json:"textPrev"` // the previous subtitle line
	Text               string    `json:"text"`
	TextNext           string    `json:"textNext"` // the next subtitle line
	StreamName         string    `json:"streamName"`
	StreamStartTime    time.Time `json:"streamStartTime"`
	StreamEndTime      time.Time `json:"streamEndTime"`
	CourseName         string    `json:"courseName"`
	CourseSlug         string    `json:"courseSlug"`
	CourseYear         int       `json:"year"`
	CourseTeachingTerm string    `json:"semester"`
}

// checkAndFillResponse takes the response of meilisearch and filters out all courses/streams/subtitles which the user is not allowed to see (checking)
// this is necessary because updating the course in the database (e.g. changing the visibility for a course from public to loggedin) does not update meilisearch data until the next day
// additionally it adds information to the results which is not saved in meili (fill)
// ---
// canSearchHiddenCourses indicates whether the response may include streams or subtitles of a hidden course
// should only be true when the user has explicitly named the hidden course he wants to search through in the url params
func checkAndFillResponse(c *gin.Context, user *model.User, limit int64, daoWrapper dao.DaoWrapper, response *meilisearch.MultiSearchResponse, canSearchHiddenCourses bool) {
	var userEligibleToSeeResultsOfHiddenCourse func(course model.Course) bool
	if canSearchHiddenCourses {
		userEligibleToSeeResultsOfHiddenCourse = user.IsEligibleToWatchCourse
	} else {
		userEligibleToSeeResultsOfHiddenCourse = user.IsEligibleToSearchForCourse
	}

	for i, res := range response.Results {
		switch res.IndexUID {
		case "STREAMS":
			hits := res.Hits
			res.Hits = []any{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliStreams []SearchStreamDTO
			temp, err := json.Marshal(hits)
			if err != nil { //shouldn't happen
				continue
			}
			err = json.Unmarshal(temp, &meiliStreams)
			if err != nil { // shouldn't happen
				continue
			}

			for _, meiliStream := range meiliStreams {
				stream, err := daoWrapper.StreamsDao.GetStreamByID(c, strconv.Itoa(int(meiliStream.ID)))
				if err != nil {
					continue
				}
				course, err := daoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)
				if err != nil {
					continue
				}

				meiliStream.CourseSlug = course.Slug
				if userEligibleToSeeResultsOfHiddenCourse(course) && (!stream.Private || user.IsAdminOfCourse(course)) {
					res.Hits = append(res.Hits, meiliStream)
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}
			response.Results[i] = res
		case "COURSES":
			hits := res.Hits
			res.Hits = []any{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliCourses []SearchCourseDTO
			temp, err := json.Marshal(hits)
			if err != nil { //shouldn't happen
				continue
			}
			err = json.Unmarshal(temp, &meiliCourses)
			if err != nil { // shouldn't happen
				continue
			}

			for _, meiliCourse := range meiliCourses {
				course, err := daoWrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, meiliCourse.Slug, meiliCourse.TeachingTerm, meiliCourse.Year)
				if err == nil && user.IsEligibleToSearchForCourse(course) {
					res.Hits = append(res.Hits, meiliCourse)
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}
			response.Results[i] = res
		case "SUBTITLES":
			hits := res.Hits
			res.Hits = []any{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliSubtitles []SearchSubtitlesDTO
			temp, err := json.Marshal(hits)
			if err != nil { //shouldn't happen
				continue
			}
			err = json.Unmarshal(temp, &meiliSubtitles)
			if err != nil { // shouldn't happen
				continue
			}

			for _, meiliSubtitle := range meiliSubtitles {
				stream, err := daoWrapper.StreamsDao.GetStreamByID(c, strconv.Itoa(int(meiliSubtitle.StreamID)))
				if err != nil {
					continue
				}
				course, err := daoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)
				if err != nil {
					continue
				}

				meiliSubtitle.StreamName = stream.Name
				meiliSubtitle.StreamStartTime = stream.Start
				meiliSubtitle.StreamEndTime = stream.End
				meiliSubtitle.CourseSlug = course.Slug
				meiliSubtitle.CourseName = course.Name
				meiliSubtitle.CourseYear = course.Year
				meiliSubtitle.CourseTeachingTerm = course.TeachingTerm
				if userEligibleToSeeResultsOfHiddenCourse(course) && (!stream.Private || user.IsAdminOfCourse(course)) {
					res.Hits = append(res.Hits, meiliSubtitle)
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}
			response.Results[i] = res
		default:
			continue
		}
	}
}

// meilisearch filter

func meiliSubtitleFilter(user *model.User, courses []model.Course) string {
	if len(courses) == 0 {
		return ""
	}

	var streamIDs []uint
	for _, course := range courses {
		admin := user.IsAdminOfCourse(course)
		for _, stream := range course.Streams {
			if !stream.Private || admin {
				streamIDs = append(streamIDs, stream.ID)
			}
		}
	}
	return fmt.Sprintf("streamID IN %s", uintSliceToString(streamIDs))
}

func meiliStreamFilter(c *gin.Context, user *model.User, semester model.Semester, courses []model.Course) string {
	if courses != nil {
		return fmt.Sprintf("courseID IN %s", courseSliceToString(courses))
	}

	semesterFilter := fmt.Sprintf("(year = %d AND semester = \"%s\")", semester.Year, semester.TeachingTerm)
	if user != nil && user.Role == model.AdminType {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = \"public\" AND private = 0)"
	} else {
		enrolledCourses := user.CoursesForSemestersWithoutAdministeredCourses(semester, semester, nil, c)
		enrolledCoursesFilter := courseSliceToString(enrolledCourses)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("((visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND courseID IN %s)) AND private = 0)", enrolledCoursesFilter)
		} else {
			administeredCourses := user.AdministeredCoursesForSemesters(semester, semester, nil, c)
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("((visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND courseID IN %s)) AND private = 0 OR courseID IN %s)", enrolledCoursesFilter, administeredCoursesFilter)
		}
	}

	if permissionFilter == "" {
		return semesterFilter
	} else {
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	}
}

func meiliCourseFilter(c *gin.Context, user *model.User, firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	semesterFilter := meiliSemesterFilter(firstSemester, lastSemester, semesters)
	if user != nil && user.Role == model.AdminType {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = \"public\")"
	} else {
		enrolledCourses := user.CoursesForSemestersWithoutAdministeredCourses(firstSemester, lastSemester, semesters, c)
		enrolledCoursesFilter := courseSliceToString(enrolledCourses)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("(visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND ID IN %s))", enrolledCoursesFilter)
		} else {
			administeredCourses := user.AdministeredCoursesForSemesters(firstSemester, lastSemester, semesters, c)
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("(visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND ID IN %s) OR ID IN %s)", enrolledCoursesFilter, administeredCoursesFilter)
		}
	}

	if semesterFilter == "" || permissionFilter == "" {
		return permissionFilter + semesterFilter
	} else {
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	}
}

func meiliSemesterFilter(firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	if len(semesters) == 0 && firstSemester.Year < 1900 && lastSemester.Year > 2800 {
		return ""
	}

	if semesters == nil {
		//single semester
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			return fmt.Sprintf("(year = %d AND semester = \"%s\")", firstSemester.Year, firstSemester.TeachingTerm)
		}

		//multiple semesters
		var constraint1, constraint2 string
		if firstSemester.TeachingTerm == "W" {
			constraint1 = fmt.Sprintf("(year = %d AND semester = \"%s\")", firstSemester.Year, firstSemester.TeachingTerm)
		} else {
			constraint1 = fmt.Sprintf("year = %d", firstSemester.Year)
		}
		if lastSemester.TeachingTerm == "S" {
			constraint2 = fmt.Sprintf("(year = %d AND semester = \"%s\")", lastSemester.Year, lastSemester.TeachingTerm)
		} else {
			constraint2 = fmt.Sprintf("year = %d", lastSemester.Year)
		}
		if firstSemester.Year+1 < lastSemester.Year {
			return fmt.Sprintf("(%s OR (year > %d AND year < %d) OR %s)", constraint1, firstSemester.Year, lastSemester.Year, constraint2)
		} else {
			return fmt.Sprintf("(%s OR %s)", constraint1, constraint2)
		}
	} else {
		semesterStringsSlice := make([]string, len(semesters))
		for i, semester := range semesters {
			semesterStringsSlice[i] = fmt.Sprintf("(year = %d AND semester = \"%s\")", semester.Year, semester.TeachingTerm)
		}
		filter := "(" + strings.Join(semesterStringsSlice, " OR ") + ")"
		return filter
	}
}

// Utility functions

func parseSemesters(semestersParam string) ([]model.Semester, error) {
	if semestersParam == "" {
		return nil, errors.New("empty semestersParam")
	}
	semesterStrings := strings.Split(semestersParam, ",")

	regex, err := regexp.Compile(`^[0-9]{4}[WS]$`)
	if err != nil {
		return nil, err
	}

	semesters := make([]model.Semester, len(semesterStrings))
	for i, semester := range semesterStrings {
		if regex.MatchString(semester) {
			year, _ := strconv.Atoi(semester[:4])
			semesters[i] = model.Semester{
				TeachingTerm: semester[4:],
				Year:         year,
			}
		} else {
			return nil, errors.New(fmt.Sprintf("semester %s is not valid", semester))
		}
	}
	return semesters, nil
}

// parses the URL Parameter course (urlParamCourse) and returns a slice containing every course in the parameter or an error code
func parseCourses(c *gin.Context, daoWrapper dao.DaoWrapper, urlParamCourse string) ([]model.Course, uint) {
	coursesStrings := strings.Split(urlParamCourse, ",")

	regex, err := regexp.Compile(`^.+[0-9]{4}[WS]$`)
	if err != nil {
		return nil, 2
	}

	courses := make([]model.Course, len(coursesStrings))
	for i, courseString := range coursesStrings {
		if !regex.MatchString(courseString) {
			return nil, 1
		}
		length := len(courseString)
		year, _ := strconv.Atoi(courseString[length-5 : length-1])
		course, err := daoWrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, courseString[:length-5], courseString[length-1:], year)
		if err != nil {
			return nil, 1
		}
		courses[i] = course
	}
	return courses, 0
}

func courseSliceToString(courses []model.Course) string {
	if courses == nil || len(courses) == 0 {
		return "[]"
	}
	idsStringSlice := make([]string, len(courses))
	for i, c := range courses {
		idsStringSlice[i] = strconv.Itoa(int(c.ID))
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}

func uintSliceToString(ids []uint) string {
	if ids == nil || len(ids) == 0 {
		return "[]"
	}
	idsStringSlice := make([]string, len(ids))
	for i, id := range ids {
		idsStringSlice[i] = strconv.FormatUint(uint64(id), 10)
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}

func ToSearchCourseDTO(cs ...model.Course) []SearchCourseDTO {
	res := make([]SearchCourseDTO, len(cs))
	for i, c := range cs {
		res[i] = SearchCourseDTO{
			Name:         c.Name,
			Slug:         c.Slug,
			Year:         c.Year,
			TeachingTerm: c.TeachingTerm,
		}
	}
	return res
}

// ToSearchStreamDTO ignores any errors and sets affected fields to zero value
func ToSearchStreamDTO(wrapper dao.DaoWrapper, streams ...model.Stream) []SearchStreamDTO {
	res := make([]SearchStreamDTO, len(streams))
	for i, s := range streams {
		var courseName, teachingTerm, slug string
		var year int
		c, err := wrapper.GetCourseById(context.Background(), s.CourseID)
		if err == nil {
			courseName = c.Name
			teachingTerm = c.TeachingTerm
			slug = c.Slug
			year = c.Year
		}
		res[i] = SearchStreamDTO{
			ID:           s.ID,
			Name:         s.Name,
			Description:  s.Description,
			CourseName:   courseName,
			Year:         year,
			TeachingTerm: teachingTerm,
			CourseSlug:   slug,
		}
	}
	return res
}

// ToSearchSubtitleDTO ignores any errors and sets affected fields to zero value
func ToSearchSubtitleDTO(wrapper dao.DaoWrapper, subtitles ...tools.MeiliSubtitles) []SearchSubtitlesDTO {
	res := make([]SearchSubtitlesDTO, len(subtitles))
	for i, subtitle := range subtitles {
		var streamName, courseName, slug, teachingTerm string
		var year int
		var startTime, endTime time.Time
		s, err := wrapper.GetStreamByID(context.Background(), strconv.Itoa(int(subtitle.StreamID)))
		if err == nil {
			c, err := wrapper.GetCourseById(context.Background(), s.CourseID)
			if err == nil {
				streamName = s.Name
				startTime = s.Start
				endTime = s.End
				courseName = c.Name
				teachingTerm = c.TeachingTerm
				slug = c.Slug
				year = c.Year
			}
		}
		res[i] = SearchSubtitlesDTO{
			StreamID:           subtitle.StreamID,
			Timestamp:          subtitle.Timestamp,
			TextPrev:           subtitle.TextPrev,
			Text:               subtitle.Text,
			TextNext:           subtitle.TextNext,
			StreamName:         streamName,
			StreamStartTime:    startTime,
			StreamEndTime:      endTime,
			CourseName:         courseName,
			CourseSlug:         slug,
			CourseYear:         year,
			CourseTeachingTerm: teachingTerm,
		}
	}
	return res
}
