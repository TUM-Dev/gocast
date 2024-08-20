package api

import (
	"context"
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
semester=...

firstSemester=...&lastSemester=...
semester=...,...,

courseID=...



*/

// TODO param check?
// TODO after search eligibility check
func (r searchRoutes) search(c *gin.Context) {
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	query := c.Query("q")
	limit, err := strconv.ParseUint(c.Query("limit"), 10, 64)
	if err != nil {
		limit = 10
	}
	var res *meilisearch.MultiSearchResponse

	if courseIDParam := c.Query("courseID"); courseIDParam != "" {
		if courseID, err := strconv.Atoi(courseIDParam); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		} else {
			// course search
			c.JSON(http.StatusOK, &courseID) //dummy

		}
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
			res = tools.Search(query, int64(limit), 6, courseFilter(c, user, firstSemester, firstSemester, nil), streamFilter(c, user, firstSemester))
		} else {
			// multiple semester search
			res = tools.Search(query, int64(limit), 4, courseFilter(c, user, firstSemester, lastSemester, nil), "")
		}
		//TODO response check
		c.JSON(http.StatusOK, res)
	}
}

// meilisearch filter

func subtitleFilter(user *model.User, courses []model.Course) string {
	if len(courses) == 0 {
		return ""
	}

	var streamIDs []uint
	for _, course := range courses {
		if user.IsEligibleToWatchCourse(course) {
			for _, stream := range course.Streams {
				if !stream.Private || user.IsAdminOfCourse(course) {
					streamIDs = append(streamIDs, stream.ID)
				}
			}
		}
	}
	return uintSliceToString(streamIDs)
}

func streamFilter(c *gin.Context, user *model.User, semester model.Semester) string {
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
	c.JSON(http.StatusOK, tools.SearchCourses(q, filter))
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

	regex, err := regexp.Compile("[0-9]{4}[WS]")
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

func courseSliceToString(courses []model.Course) string {
	if courses == nil || len(courses) == 0 {
		return "[]"
	}
	var idsStringSlice []string
	idsStringSlice = make([]string, len(courses))
	for i, c := range courses {
		idsStringSlice[i] = strconv.FormatUint(uint64(c.ID), 10)
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}

func uintSliceToString(ids []uint) string {
	if ids == nil || len(ids) == 0 {
		return "[]"
	}
	var idsStringSlice []string
	idsStringSlice = make([]string, len(ids))
	for i, id := range ids {
		idsStringSlice[i] = strconv.FormatUint(uint64(id), 10)
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}
