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
		c.AbortWithStatus(http.StatusBadRequest)
		return
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

	if semestersParam := c.Query("semester"); semestersParam != "" {
		if semesters, err := parseSemesters(semestersParam); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		} else {
			if len(*semesters) == 1 {
				// one semester search
			} else {
				// multiple semesters search
			}
		}
	}
	c.JSON(http.StatusOK, fmt.Sprintf("%s%s%d", user.Name, query, limit)) //dummy
}

func parseSemesters(semestersParam string) (*[]dao.Semester, error) {
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
	return &semesters, nil
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
	return uintArrayToString(&streamIDs)
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
		return fmt.Sprintf("((visibility = loggedin OR visibility = public OR (visibility = enrolled AND courseID in %s)) AND private = 0)", courseIdFilter(c, user, semester, semester))
	} else {
		return ""
	}
}

func courseFilter(c *gin.Context, user *model.User, firstSemester dao.Semester, lastSemester dao.Semester) string {
	semesterFilter := semesterFilter(firstSemester, lastSemester)
	if user == nil || user.Role != model.AdminType {
		permissionFilter := coursePermissionFilter(c, user, firstSemester, lastSemester)
		return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
	} else {
		return semesterFilter
	}
}

func semesterFilter(firstSemester dao.Semester, lastSemester dao.Semester) string {
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
		return fmt.Sprintf("(visibility = loggedin OR visibility = public OR (visibility = enrolled AND ID IN %s))", courseIdFilter(c, user, firstSemester, lastSemester))
	} else {
		return ""
	}
}

// returns a string conforming to MeiliSearch filter format containing each courseId passed onto the function
func courseIdFilter(c *gin.Context, user *model.User, firstSemester dao.Semester, lastSemester dao.Semester) string {
	courses := make([]model.Course, 0)
	if user != nil {
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			courses = user.CoursesForSemester(firstSemester.Year, firstSemester.TeachingTerm, c)
		} else {
			courses = user.CoursesForSemesters(firstSemester.Year, firstSemester.TeachingTerm, lastSemester.Year, lastSemester.TeachingTerm, c)
		}
	}
	courseIDs := make([]uint, 0)
	for _, course := range courses {
		courseIDs = append(courseIDs, course.ID)
	}

	return uintArrayToString(&courseIDs)
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
	var courseIDsAsStringArray []string
	courseIDsAsStringArray = make([]string, len(*courseIDs))
	for i, courseID := range *courseIDs {
		courseIDsAsStringArray[i] = strconv.FormatUint(uint64(courseID), 10)
	}
	courses := "[" + strings.Join(courseIDsAsStringArray, ", ") + "]"
	sem := dao.Semester{
		TeachingTerm: t,
		Year:         int(y),
	}
	filter := fmt.Sprintf("%s AND ID IN %s", semesterFilter(sem, sem), courses)
	c.JSON(http.StatusOK, tools.SearchCourses(q, filter))
}

func (r searchRoutes) getSearchableCoursesOfUserForOneSemester(c *gin.Context, user *model.User, y int64, t string) *[]uint {
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
	return &courseIDs
}

func uintArrayToString(ids *[]uint) string {
	if len(*ids) == 0 {
		return ""
	}
	var idsAsStringArray []string
	idsAsStringArray = make([]string, len(*ids))
	for i, id := range *ids {
		idsAsStringArray[i] = strconv.FormatUint(uint64(id), 10)
	}
	filter := "[" + strings.Join(idsAsStringArray, ", ") + "]"
	return filter
}
