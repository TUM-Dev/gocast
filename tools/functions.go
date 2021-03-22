package tools

import "TUM-Live/model"

/*
 * not terribly efficient, might use a map later, but as every user only has a handful of courses fast enough
 */
func CourseListContains(courses []model.Course, courseId uint) bool {
	if courses != nil {
		for _, c := range courses {
			if c.ID == courseId {
				return true
			}
		}
	}
	return false
}
