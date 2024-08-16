package api

import (
	"context"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
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
		res := tools.Search(query, int64(limit), 4, courseFilter(c, user, firstSemester, lastSemester), "")
		//TODO response check

		return
	}

	semestersParam := c.Query("semester")
	if semestersParam != "" {
		if semesters, err := parseSemesters(semestersParam); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		} else {
			if len(semesters) == 1 {
				// one semester search
				res := tools.Search(query, int64(limit), 6, courseFilter(c, user, semesters[0], semesters[0]), streamFilter(c, user, semesters[0]))
			} else {
				// multiple semesters search
				res := tools.Search(query, int64(limit), 4, courseFilter(c, user, semesters[0], semesters[1]), "")
			}
		}
	}
	c.JSON(http.StatusOK, fmt.Sprintf("%s%s%d", user.Name, query, limit)) //dummy
}

func parseSemesters(semestersParam string) ([]dao.Semester, error) {
	semesterStrings := strings.Split(semestersParam, ",")

	regex, err := regexp.Compile("[0-9]{4}[WS]")
	if err != nil {
		return nil, err
	}

	semesters := make([]dao.Semester, len(semesterStrings))
	for _, semester := range semesterStrings {
		if year, err := strconv.Atoi(semester[:4]); regex.MatchString(semestersParam) && err == nil {
			semesters = append(semesters, dao.Semester{
				TeachingTerm: semester[4:],
				Year:         year,
			})
		} else {
			return nil, err
		}
	}
	return semesters, nil
}

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

func streamFilter(c *gin.Context, user *model.User, semester dao.Semester) string {
	semesterFilter := fmt.Sprintf("(year = %d AND teachingTerm = %s)", semester.Year, semester.TeachingTerm)
	if user == nil || user.Role != model.AdminType {
		permissionFilter := streamPermissionFilter(c, user, semester)
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	} else {
		return semesterFilter
	}
}

// TODO private streams searchable for course admins
// TODO mit coursePermissionFilter zusammenlegen
func streamPermissionFilter(c *gin.Context, user *model.User, semester dao.Semester) string {
	if user == nil {
		return "(visibility = public AND private = 0)"
	} else if user.Role != model.AdminType {
		if len(user.AdministeredCourses) == 0 {
			return fmt.Sprintf("((visibility = loggedin OR visibility = public OR (visibility = enrolled AND courseID in %s)) AND private = 0)", courseIdFilter(c, user, semester, semester))
		} else {
			administeredCourses := user.AdministeredCoursesForSemester(semester.Year, semester.TeachingTerm, c)
			var administeredCourseIDs []uint
			for _, course := range administeredCourses {
				administeredCourseIDs = append(administeredCourseIDs, course.ID)
			}
			administeredCoursesFilter := uintSliceToString(administeredCourseIDs)
			return fmt.Sprintf("((visibility = loggedin OR visibility = public OR (visibility = enrolled AND courseID in %s)) AND private = 0 OR courseID IN %s)", courseIdFilter(c, user, semester, semester), administeredCoursesFilter)

		}
	} else {
		return ""
	}
}

func courseFilter(c *gin.Context, user *model.User, firstSemester dao.Semester, lastSemester dao.Semester) string {
	semesterFilter := meiliSemesterFilterInRange(firstSemester, lastSemester)
	if user == nil || user.Role != model.AdminType {
		permissionFilter := coursePermissionFilter(c, user, firstSemester, lastSemester)
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	} else {
		return semesterFilter
	}
}

func meiliSemesterFilterInRange(firstSemester dao.Semester, lastSemester dao.Semester) string {
	if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
		return fmt.Sprintf("(year = %d AND teachingTerm = %s)", firstSemester.Year, firstSemester.TeachingTerm)
	} else {
		var constraint1, constraint2 string
		if firstSemester.TeachingTerm == "W" {
			constraint1 = fmt.Sprintf("(year = %d AND teachingTerm = %s)", firstSemester.Year, firstSemester.TeachingTerm)
		} else {
			constraint1 = fmt.Sprintf("year = %d", firstSemester.Year)
		}
		if lastSemester.TeachingTerm == "S" {
			constraint2 = fmt.Sprintf("(year = %d AND teachingTerm = %s)", lastSemester.Year, lastSemester.TeachingTerm)
		} else {
			constraint2 = fmt.Sprintf("year = %d", lastSemester.Year)
		}
		if firstSemester.Year+1 < lastSemester.Year {
			return fmt.Sprintf("(%s OR (year > %d AND year < %d) OR %s)", constraint1, firstSemester.Year, lastSemester.Year, constraint2)
		} else {
			return fmt.Sprintf("(%s OR %s)", constraint1, constraint2)
		}
	}
}

// TODO OR ID in [administeredcourses]
func coursePermissionFilter(c *gin.Context, user *model.User, firstSemester dao.Semester, lastSemester dao.Semester) string {
	if user == nil {
		return "(visibility = public)"
	} else if user.Role != model.AdminType {
		if len(user.AdministeredCourses) == 0 {
			return fmt.Sprintf("(visibility = loggedin OR visibility = public OR (visibility = enrolled AND ID IN %s))", courseIdFilter(c, user, firstSemester, lastSemester))
		} else {
			administeredCourses := user.AdministeredCoursesForSemesters(firstSemester.Year, firstSemester.TeachingTerm, lastSemester.Year, lastSemester.TeachingTerm, c)
			var administeredCourseIDs []uint
			for _, course := range administeredCourses {
				administeredCourseIDs = append(administeredCourseIDs, course.ID)
			}
			administeredCoursesFilter := uintSliceToString(administeredCourseIDs)
			return fmt.Sprintf("(visibility = loggedin OR visibility = public OR (visibility = enrolled AND ID IN %s) OR ID in %s)", courseIdFilter(c, user, firstSemester, lastSemester), administeredCoursesFilter)
		}
	} else {
		return ""
	}
}

// returns a string conforming to MeiliSearch filter format containing each courseId passed onto the function
func courseIdFilter(c *gin.Context, user *model.User, firstSemester dao.Semester, lastSemester dao.Semester) string {
	courses := make([]model.Course, 0)
	courseIDs := make([]uint, 0)
	if user != nil {
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			courses = user.CoursesForSemesterWithoutAdministeredCourses(firstSemester.Year, firstSemester.TeachingTerm, c)
		} else {
			courses = user.CoursesForSemestersWithoutAdministeredCourses(firstSemester.Year, firstSemester.TeachingTerm, lastSemester.Year, lastSemester.TeachingTerm, c)
		}
		for _, c := range courses {
			courseIDs = append(courseIDs, c.ID)
		}
	}
	return uintSliceToString(courseIDs)
}

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
	sem := dao.Semester{
		TeachingTerm: t,
		Year:         int(y),
	}
	filter := fmt.Sprintf("%s AND ID IN %s", meiliSemesterFilterInRange(sem, sem), uintSliceToString(courseIDs))
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

func uintSliceToString(ids []uint) string {
	if ids == nil || len(ids) == 0 {
		return "[]"
	}
	var idsAsStringArray []string
	idsAsStringArray = make([]string, len(ids))
	for i, id := range ids {
		idsAsStringArray[i] = strconv.FormatUint(uint64(id), 10)
	}
	filter := "[" + strings.Join(idsAsStringArray, ", ") + "]"
	return filter
}
