package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"github.com/meilisearch/meilisearch-go"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := searchRoutes{daoWrapper}

	searchGroup := router.Group("/api/search")
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
}

func (r searchRoutes) searchSubtitles(c *gin.Context) {
	s := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).Stream
	q := c.Query("q")
	c.JSON(http.StatusOK, tools.SearchSubtitles(q, s.ID))
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

func (r searchRoutes) search(c *gin.Context) {
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	query := c.Query("q")
	limit, err := strconv.ParseUint(c.Query("limit"), 10, 64)
	if err != nil {
		limit = 10
	}
	var res *meilisearch.MultiSearchResponse

	if courseParam := c.Query("course"); courseParam != "" {
		courses, errorCode := parseCourses(c, r.DaoWrapper, courseParam)
		if errorCode == 2 {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if errorCode != 0 || len(courses) > 5 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		for _, course := range courses {
			if !user.IsEligibleToWatchCourse(course) {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}
		checkResponse(c, user, int64(limit), r.DaoWrapper, res)
		c.JSON(http.StatusOK, tools.Search(query, int64(limit), 3, "", streamFilter(c, user, model.Semester{}, courses), subtitleFilter(user, courses)))
		return
	}

	firstSemesterParam := c.Query("firstSemester")
	lastSemesterParam := c.Query("lastSemester")
	if firstSemesterParam != "" && lastSemesterParam != "" {
		semesters1, err1 := parseSemesters(firstSemesterParam)
		semesters2, err2 := parseSemesters(lastSemesterParam)
		if err1 != nil || err2 != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		firstSemester := semesters1[0]
		lastSemester := semesters2[0]

		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			// single semester search
			res = tools.Search(query, int64(limit), 6, courseFilter(c, user, firstSemester, firstSemester, nil), streamFilter(c, user, firstSemester, nil), "")
		} else {
			// multiple semester search
			res = tools.Search(query, int64(limit), 4, courseFilter(c, user, firstSemester, lastSemester, nil), "", "")
		}
		checkResponse(c, user, int64(limit), r.DaoWrapper, res)
		c.JSON(http.StatusOK, res)
		return
	}

	semesterParam := c.Query("semester")
	if semesterParam != "" {
		semesters, err := parseSemesters(semesterParam)
		if err != nil || len(semesters) > 5 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if len(semesters) == 1 {
			res = tools.Search(query, int64(limit), 6, courseFilter(c, user, model.Semester{}, model.Semester{}, semesters), streamFilter(c, user, semesters[0], nil), "")
		} else {
			res = tools.Search(query, int64(limit), 4, courseFilter(c, user, model.Semester{}, model.Semester{}, semesters), "", "")
		}
		checkResponse(c, user, int64(limit), r.DaoWrapper, res)
		c.JSON(http.StatusOK, res)
		return
	}
}

func checkResponse(c *gin.Context, user *model.User, limit int64, daoWrapper dao.DaoWrapper, response *meilisearch.MultiSearchResponse) {
	type MeiliResponseCourse struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Year         int    `json:"year"`
		TeachingTerm string `json:"semester"`
	}
	type MeiliResponseStream struct {
		ID           uint   `json:"ID"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		CourseName   string `json:"courseName"`
		Year         int    `json:"year"`
		TeachingTerm string `json:"semester"`
	}

	for _, res := range response.Results {
		switch res.IndexUID {
		case "STREAMS":
			var meiliStreams []MeiliResponseStream
			temp, err := json.Marshal(res.Hits) //TODO use res.MarshalJSON ?
			if err != nil {                     //shouldn't happen
				res.Hits = make([]interface{}, 0)
				continue
			}
			err = json.Unmarshal(temp, &meiliStreams)
			if err != nil { // shouldn't happen
				res.Hits = make([]interface{}, 0)
				continue
			}

			res.Hits = []interface{}{}
			for _, meiliStream := range meiliStreams {
				stream, err := daoWrapper.StreamsDao.GetStreamByID(c, strconv.Itoa(int(meiliStream.ID))) //TODO kann deleted streams returnen
				if err != nil {
					continue
				}
				course, err := daoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)
				if err != nil {
					continue
				}
				if user.IsEligibleToWatchCourse(course) && !stream.Private || user.IsAdminOfCourse(course) {
					res.Hits = append(res.Hits, meiliStream)
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}

		case "COURSES":
			var meiliCourses []MeiliResponseCourse
			temp, err := json.Marshal(res.Hits) //TODO use res.MarshalJSON ?
			if err != nil {                     //shouldn't happen
				res.Hits = make([]interface{}, 0)
				continue
			}
			err = json.Unmarshal(temp, &meiliCourses)
			if err != nil { // shouldn't happen
				res.Hits = make([]interface{}, 0)
				continue
			}

			res.Hits = []interface{}{}
			for _, meiliCourse := range meiliCourses {
				course, err := daoWrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, meiliCourse.Slug, meiliCourse.TeachingTerm, meiliCourse.Year) //TODO kann deleted courses returnen
				if err == nil && user.IsEligibleToWatchCourse(course) {
					res.Hits = append(res.Hits, meiliCourse)
				}
				if len(res.Hits) >= int(limit) {
					break
				}
			}
		default:
			continue
		}
	}
}

// meilisearch filter

func subtitleFilter(user *model.User, courses []model.Course) string {
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

func streamFilter(c *gin.Context, user *model.User, semester model.Semester, courses []model.Course) string {
	if courses != nil {
		return fmt.Sprintf("courseID IN %s", courseSliceToString(courses))
	}

	semesterFilter := fmt.Sprintf("(year = %d AND semester = %s)", semester.Year, semester.TeachingTerm)
	if user != nil && user.Role == model.AdminType {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = public AND private = 0)"
	} else {
		filter := courseIdFilter(c, user, semester, semester, nil)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("((visibility = loggedin OR visibility = public OR (visibility = enrolled AND courseID in %s)) AND private = 0)", filter)
		} else {
			administeredCourses := user.AdministeredCoursesForSemester(semester.Year, semester.TeachingTerm, c)
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("((visibility = loggedin OR visibility = public OR (visibility = enrolled AND courseID in %s)) AND private = 0 OR courseID IN %s)", filter, administeredCoursesFilter)
		}
	}

	if permissionFilter == "" {
		return semesterFilter
	} else {
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	}
}

func courseFilter(c *gin.Context, user *model.User, firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	semesterFilter := meiliSemesterFilter(firstSemester, lastSemester, semesters)
	if user != nil && user.Role == model.AdminType {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = public)"
	} else {
		filter := courseIdFilter(c, user, firstSemester, lastSemester, semesters)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("(visibility = loggedin OR visibility = public OR (visibility = enrolled AND ID IN %s))", filter)
		} else {
			administeredCourses := user.AdministeredCoursesForSemesters(firstSemester, lastSemester, semesters, c)
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("(visibility = loggedin OR visibility = public OR (visibility = enrolled AND ID IN %s) OR ID in %s)", filter, administeredCoursesFilter)
		}
	}

	if semesterFilter == "" || permissionFilter == "" {
		return permissionFilter + semesterFilter
	} else {
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	}
}

func meiliSemesterFilter(firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	if semesters == nil {
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			return fmt.Sprintf("(year = %d AND semester = %s)", firstSemester.Year, firstSemester.TeachingTerm)
		} else if len(semesters) == 1 {
			return fmt.Sprintf("(year = %d AND semester = %s)", semesters[0].Year, semesters[0].TeachingTerm)
		} else {
			var constraint1, constraint2 string
			if firstSemester.TeachingTerm == "W" {
				constraint1 = fmt.Sprintf("(year = %d AND semester = %s)", firstSemester.Year, firstSemester.TeachingTerm)
			} else {
				constraint1 = fmt.Sprintf("year = %d", firstSemester.Year)
			}
			if lastSemester.TeachingTerm == "S" {
				constraint2 = fmt.Sprintf("(year = %d AND semester = %s)", lastSemester.Year, lastSemester.TeachingTerm)
			} else {
				constraint2 = fmt.Sprintf("year = %d", lastSemester.Year)
			}
			if firstSemester.Year+1 < lastSemester.Year {
				return fmt.Sprintf("(%s OR (year > %d AND year < %d) OR %s)", constraint1, firstSemester.Year, lastSemester.Year, constraint2)
			} else {
				return fmt.Sprintf("(%s OR %s)", constraint1, constraint2)
			}
		}
	} else {
		if len(semesters) == 0 {
			return ""
		}
		semesterStringsSlice := make([]string, len(semesters))
		for i, semester := range semesters {
			semesterStringsSlice[i] = fmt.Sprintf("(year = %d AND semester = %s)", semester.Year, semester.TeachingTerm)
		}
		filter := "(" + strings.Join(semesterStringsSlice, " OR ") + ")"
		return filter
	}
}

// params that are not used may be of corresponding zero value
// returns a meili array representation of all courseIDs the user is allowed to search for
func courseIdFilter(c *gin.Context, user *model.User, firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	courses := make([]model.Course, 0)
	if user != nil {
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			courses = user.CoursesForSemesterWithoutAdministeredCourses(firstSemester.Year, firstSemester.TeachingTerm, c)
		} else if len(semesters) == 1 {
			courses = user.CoursesForSemesterWithoutAdministeredCourses(semesters[0].Year, semesters[0].TeachingTerm, c)
		} else {
			courses = user.CoursesForSemestersWithoutAdministeredCourses(firstSemester, lastSemester, semesters, c)
		}
	}
	return courseSliceToString(courses)
}

// Old searchCourses route

// TODO refactor to match function search
func (r searchRoutes) searchCourses(c *gin.Context) {
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	q := c.Query("q")
	t := c.Query("term")
	y, err := strconv.ParseInt(c.Query("year"), 10, 64)
	if err != nil || (t != "W" && t != "S") {
		return
	}

	courseIDs := r.getSearchableCoursesOfUserForOneSemester(c, user, y, t)
	sem := model.Semester{
		TeachingTerm: t,
		Year:         int(y),
	}
	filter := fmt.Sprintf("%s AND ID IN %s", meiliSemesterFilter(sem, sem, nil), uintSliceToString(courseIDs))

	type MeiliResponseCourse struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Year         int    `json:"year"`
		TeachingTerm string `json:"semester"`
	}
	var _ MeiliResponseCourse
	res := tools.SearchCourses(q, filter)
	z, _ := json.Marshal(res.Hits)
	println(string(z))
	for _, hit := range res.Hits {
		switch hit.(type) {
		case map[string]interface{}:
			// println("yippiee")
			var _ map[int]interface{}
			//res.Hits[0] = sdf
		}
		var m []MeiliResponseCourse
		x, _ := json.Marshal(res.Hits)
		_ = json.Unmarshal(x, &m)
		//println(m[0].Name)
		res.Hits[0] = m[0]
	}
	z, _ = json.Marshal(res.Hits)
	println(string(z))

	c.JSON(http.StatusOK, res)
}

func (r searchRoutes) getSearchableCoursesOfUserForOneSemester(c *gin.Context, user *model.User, y int64, t string) []uint {
	var courses []model.Course
	if user != nil {
		switch user.Role {
		case model.AdminType:
			courses = r.GetAllCoursesForSemester(int(y), t, c)
		default: // user.CoursesForSemesters includes both Administered Courses and enrolled Courses
			courses, _ = r.CoursesDao.GetPublicAndLoggedInCourses(int(y), t)
			courses = append(courses, user.CoursesForSemester(int(y), t, context.Background())...)
		}
	} else {
		courses, _ = r.GetPublicCourses(int(y), t)
	}

	distinctCourseIDs := make(map[uint]bool)
	var courseIDs []uint
	for _, course := range courses {
		value := distinctCourseIDs[course.ID]
		if !value {
			courseIDs = append(courseIDs, course.ID)
			distinctCourseIDs[course.ID] = true
		}
	}
	return courseIDs
}

// Utility functions

func parseSemesters(semestersParam string) ([]model.Semester, error) {
	semesterStrings := strings.Split(semestersParam, ",")

	regex, err := regexp.Compile(`^[0-9]{4}[WS]$`)
	if err != nil {
		return nil, err
	}

	semesters := make([]model.Semester, len(semesterStrings))
	for _, semester := range semesterStrings {
		if year, err := strconv.Atoi(semester[:4]); regex.MatchString(semestersParam) && err == nil {
			semesters = append(semesters, model.Semester{
				TeachingTerm: semester[4:],
				Year:         year,
			})
		} else {
			return nil, err
		}
	}
	return semesters, nil
}

func parseCourses(c *gin.Context, daoWrapper dao.DaoWrapper, coursesParam string) ([]model.Course, uint) {
	coursesStrings := strings.Split(coursesParam, ",")

	regex, err := regexp.Compile(`^.+[0-9]{4}[WS]$`)
	if err != nil {
		return nil, 2
	}

	courses := make([]model.Course, len(coursesStrings))
	for i, courseString := range coursesStrings {
		if !regex.MatchString(coursesParam) {
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
