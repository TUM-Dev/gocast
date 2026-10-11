package tum

import (
	"context"

	"github.com/TUM-Dev/gocast/dao"
)

// FetchCourses updates the enrollments of all relevant courses
func FetchCourses(daoWrapper dao.DaoWrapper) func() {
	return func() {
		y, t := GetCurrentSemester()
		courses, err := daoWrapper.CoursesDao.GetAllCoursesWithTUMIDFromSemester(context.Background(), y, t)
		if err != nil {
			logger.Error("Could not get courses with TUM online identifier:", "err", err)
			return
		}
		FindStudentsForCourses(courses, daoWrapper.UsersDao)
		// GetEventsForCourses(courses)
	}
}
