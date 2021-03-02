package dao

import (
	"TUM-Live/model"
	"context"
)

func GetCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	var foundCourses []model.Course
	dbErr := DB.Find(&foundCourses, "user_id = ?", userid).Error
	return foundCourses, dbErr
}
