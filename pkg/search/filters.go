package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"

	"github.com/TUM-Dev/gocast/apiv2/visibility"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

// FilterMaxCoursesCount caps how many courses one search may be limited to.
const FilterMaxCoursesCount = 3

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
func checkAndFillResponse(c context.Context, user *model.User, limit int64, daoWrapper dao.DaoWrapper, response *meilisearch.MultiSearchResponse, canSearchHiddenCourses bool) {
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
			res.Hits = []meilisearch.Hit{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliStreams []SearchStreamDTO
			temp, err := json.Marshal(hits)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response post processing streams json marshaling error", "err", err)
				continue
			}
			err = json.Unmarshal(temp, &meiliStreams)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response streams post processing streams json unmarshaling error", "err", err)
				continue
			}

			for _, meiliStream := range meiliStreams {
				stream, err := daoWrapper.StreamsDao.GetStreamByID(c, strconv.FormatUint(uint64(meiliStream.ID), 10))
				if err != nil {
					continue
				}
				course, err := daoWrapper.CoursesDao.GetCourseById(c, stream.CourseID)
				if err != nil {
					continue
				}

				meiliStream.CourseSlug = course.Slug
				if userEligibleToSeeResultsOfHiddenCourse(course) && visibility.StreamVisible(user, course, stream) {
					hit, err := dtoToHit(meiliStream)
					if err != nil {
						logger.Warn("meilisearch response post processing streams hit conversion error", "err", err)
					} else {
						res.Hits = append(res.Hits, hit)
					}
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}
			response.Results[i] = res
		case "COURSES":
			hits := res.Hits
			res.Hits = []meilisearch.Hit{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliCourses []SearchCourseDTO
			temp, err := json.Marshal(hits)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response post processing courses json marshaling error", "err", err)
				continue
			}
			err = json.Unmarshal(temp, &meiliCourses)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response post processing courses json unmarshaling error", "err", err)
				continue
			}

			for _, meiliCourse := range meiliCourses {
				course, err := daoWrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, meiliCourse.Slug, meiliCourse.TeachingTerm, meiliCourse.Year)
				if err == nil && user.IsEligibleToSearchForCourse(course) {
					hit, err := dtoToHit(meiliCourse)
					if err != nil {
						logger.Warn("meilisearch response post processing courses hit conversion error", "err", err)
					} else {
						res.Hits = append(res.Hits, hit)
					}
				}

				if len(res.Hits) >= int(limit) {
					break
				}
			}
			response.Results[i] = res
		case "SUBTITLES":
			hits := res.Hits
			res.Hits = []meilisearch.Hit{}
			response.Results[i] = meilisearch.SearchResponse{}

			var meiliSubtitles []SearchSubtitlesDTO
			temp, err := json.Marshal(hits)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response post processing subtitles json marshaling error", "err", err)
				continue
			}
			err = json.Unmarshal(temp, &meiliSubtitles)
			if err != nil { // shouldn't happen
				logger.Warn("meilisearch response post processing subtitles json unmarshaling error", "err", err)
				continue
			}

			for _, meiliSubtitle := range meiliSubtitles {
				stream, err := daoWrapper.StreamsDao.GetStreamByID(c, strconv.FormatUint(uint64(meiliSubtitle.StreamID), 10))
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
				if userEligibleToSeeResultsOfHiddenCourse(course) && visibility.StreamVisible(user, course, stream) {
					hit, err := dtoToHit(meiliSubtitle)
					if err != nil {
						logger.Warn("meilisearch response post processing subtitles hit conversion error", "err", err)
					} else {
						res.Hits = append(res.Hits, hit)
					}
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

// meilisearch filters

// meiliSubtitleFilter returns a filter conforming to MeiliSearch filter format that can be used for filtering subtitles
//
// Checking eligibility to search in each course in courses is the caller's responsibility
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

// meiliStreamFilter returns a filter conforming to MeiliSearch filter format that can be used for filtering streams
//
// Checking eligibility to search for courses and validation of model.Semester format is the caller's responsibility
func meiliStreamFilter(user *model.User, semester model.Semester, courses []model.Course) string {
	if courses != nil {
		administeredCourses := make([]model.Course, 0)
		nonAdministeredCourses := make([]model.Course, 0)
		for _, course := range courses {
			if user.IsAdminOfCourse(course) {
				administeredCourses = append(administeredCourses, course)
			} else {
				nonAdministeredCourses = append(nonAdministeredCourses, course)
			}
		}
		if user == nil || len(administeredCourses) == 0 {
			return fmt.Sprintf("courseID IN %s AND private = 0", courseSliceToString(nonAdministeredCourses))
		}
		return fmt.Sprintf("(courseID IN %s OR (courseID IN %s AND private = 0))", courseSliceToString(administeredCourses), courseSliceToString(nonAdministeredCourses))
	}

	semesterFilter := fmt.Sprintf("(year = %d AND semester = \"%s\")", semester.Year, semester.TeachingTerm)
	if user.Can(model.PermViewAllCourses) {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = \"public\" AND private = 0)"
	} else {
		enrolledCourses := user.CoursesBetweenSemestersWithoutAdministeredCourses(semester, semester)
		enrolledCoursesFilter := courseSliceToString(enrolledCourses)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("((visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND courseID IN %s)) AND private = 0)", enrolledCoursesFilter)
		} else {
			administeredCourses := user.AdministeredCoursesBetweenSemesters(semester, semester)
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("((visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND courseID IN %s)) AND private = 0 OR courseID IN %s)", enrolledCoursesFilter, administeredCoursesFilter)
		}
	}

	if permissionFilter == "" {
		return semesterFilter
	}
	return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
}

// meiliCourseFilter returns a filter conforming to MeiliSearch filter format that can be used for filtering courses
//
// # Validation of model.Semester format is the caller's responsibility
//
// ignores either semesters or firstSemester/lastSemester, depending on semesters == nil
func meiliCourseFilter(user *model.User, firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	semesterFilter := meiliSemesterFilter(firstSemester, lastSemester, semesters)
	if user.Can(model.PermViewAllCourses) {
		return semesterFilter
	}

	var permissionFilter string
	if user == nil {
		permissionFilter = "(visibility = \"public\")"
	} else {
		var enrolledCourses []model.Course
		if semesters == nil {
			enrolledCourses = user.CoursesBetweenSemestersWithoutAdministeredCourses(firstSemester, lastSemester)
		} else {
			enrolledCourses = user.CoursesForSemestersWithoutAdministeredCourses(semesters)
		}
		enrolledCoursesFilter := courseSliceToString(enrolledCourses)
		if len(user.AdministeredCourses) == 0 {
			permissionFilter = fmt.Sprintf("(visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND ID IN %s))", enrolledCoursesFilter)
		} else {
			var administeredCourses []model.Course
			if semesters == nil {
				administeredCourses = user.AdministeredCoursesBetweenSemesters(firstSemester, lastSemester)
			} else {
				administeredCourses = user.AdministeredCoursesForSemesters(semesters)
			}
			administeredCoursesFilter := courseSliceToString(administeredCourses)
			permissionFilter = fmt.Sprintf("(visibility = \"loggedin\" OR visibility = \"public\" OR (visibility = \"enrolled\" AND ID IN %s) OR ID IN %s)", enrolledCoursesFilter, administeredCoursesFilter)
		}
	}

	if semesterFilter == "" || permissionFilter == "" {
		return permissionFilter + semesterFilter
	}
	return fmt.Sprintf("(%s AND %s)", permissionFilter, semesterFilter)
}

// meiliSemesterFilter returns a filter conforming to MeiliSearch filter format
//
// # Validation of model.Semester format is the caller's responsibility
//
// ignores either semesters or firstSemester/lastSemester, depending on semesters == nil
func meiliSemesterFilter(firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) string {
	if len(semesters) == 0 && firstSemester.Year < 1900 && lastSemester.Year > 2800 {
		return ""
	}

	if semesters == nil {
		// single semester
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			return fmt.Sprintf("(year = %d AND semester = \"%s\")", firstSemester.Year, firstSemester.TeachingTerm)
		}

		// multiple semesters
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
		}
		return fmt.Sprintf("(%s OR %s)", constraint1, constraint2)
	}

	semesterStringsSlice := make([]string, len(semesters))
	for i, semester := range semesters {
		semesterStringsSlice[i] = fmt.Sprintf("(year = %d AND semester = \"%s\")", semester.Year, semester.TeachingTerm)
	}
	filter := "(" + strings.Join(semesterStringsSlice, " OR ") + ")"
	return filter
}

// parseSemesters parses the URL Parameter semester and returns a slice containing every semester in the parameter or an error code
func parseSemesters(semestersParam string) ([]model.Semester, error) {
	if semestersParam == "" {
		return nil, errors.New("empty semestersParam")
	}
	semesterStrings := strings.Split(semestersParam, ",")

	regex, err := regexp.Compile(`^[0-9]{4}[WS]$`) //nolint:all
	if err != nil {
		logger.Warn("regex compile error", "err", err)
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
			return nil, errors.New(fmt.Sprintf("semester %s is not valid", semester)) //nolint:all
		}
	}
	return semesters, nil
}

// parseCourses parses the URL Parameter course (urlParamCourse) and returns a slice containing every course in the parameter or an error code
//
// Checking if the user is allowed to see returned courses is the caller's responsibility
func parseCourses(c context.Context, daoWrapper dao.DaoWrapper, urlParamCourse string) ([]model.Course, uint) {
	coursesStrings := strings.Split(urlParamCourse, ",")

	regex, err := regexp.Compile(`^.+[0-9]{4}[WS]$`) //nolint:all
	if err != nil {
		logger.Warn("regex compiler error", "err", err)
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
	if len(courses) == 0 {
		return "[]"
	}
	idsStringSlice := make([]string, len(courses))
	for i, c := range courses {
		idsStringSlice[i] = strconv.FormatUint(uint64(c.ID), 10)
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}

func uintSliceToString(ids []uint) string {
	if len(ids) == 0 {
		return "[]"
	}
	idsStringSlice := make([]string, len(ids))
	for i, id := range ids {
		idsStringSlice[i] = strconv.FormatUint(uint64(id), 10)
	}
	filter := "[" + strings.Join(idsStringSlice, ",") + "]"
	return filter
}

func determineSingleSemester(firstSemester model.Semester, lastSemester model.Semester, semesters []model.Semester) (bool, model.Semester) {
	if semesters == nil {
		if firstSemester.IsEqual(lastSemester) {
			return true, firstSemester
		}
		return false, model.Semester{}
	}
	if len(semesters) == 1 {
		return true, semesters[0]
	}
	return false, model.Semester{}
}

func dtoToHit(v any) (meilisearch.Hit, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var h meilisearch.Hit
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, err
	}

	return h, nil
}
