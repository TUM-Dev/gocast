package tools

import "TUM-Live/model"

// CourseListContains checks whether courses contain a course with a given courseId
func CourseListContains(courses []model.Course, courseId uint) bool {
	// not terribly efficient, might use a map later, but as every user only has a handful of courses fast enough
	for _, c := range courses {
		if c.ID == courseId {
			return true
		}
	}

	return false
}
