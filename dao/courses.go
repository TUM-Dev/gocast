package dao

import (
	"TUM-Live/model"
	"context"
)

func GetCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	var foundCourses []model.Course
	dbErr := DB.Table("courses").
		Select("courses.*").
		Joins("left join course_owners co on courses.id = co.course_id").
		Where("co.user_id = ?", userid).
		Scan(&foundCourses).Error
	return foundCourses, dbErr
}
