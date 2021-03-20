package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"gorm.io/gorm"
)

func GetCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	isAdmin, err := IsUserAdmin(ctx, userid)
	if err != nil {
		return nil, err
	}
	var foundCourses []model.Course
	if isAdmin {
		dbErr := DB.Preload("Streams").Find(&foundCourses).Error
		return foundCourses, dbErr
	}
	dbErr := DB.Find(&foundCourses, "user_id = ?", userid).Error
	return foundCourses, dbErr
}

func GetCourseById(ctx context.Context, id uint) (courses model.Course, err error) {
	var foundCourse model.Course
	dbErr := DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&foundCourse, "id = ?", id).Error
	return foundCourse, dbErr
}

func GetAllCoursesWithTUMID(ctx context.Context) (courses []model.Course, err error) {
	if Logger != nil {
		Logger(ctx, "Find all courses with tum_online_identifier")
	}
	var foundCourses []model.Course
	dbErr := DB.Where("tum_online_identifier IS NOT NULL").Find(&foundCourses).Error
	if dbErr != nil {
		if Logger != nil {
			Logger(ctx, fmt.Sprintf("Unable to query courses with tum_online_identifier:%v\n", dbErr))
		}
		return nil, err
	}
	return foundCourses, nil
}

/**
* Saves all provided courses into database.
**/
func UpdateCourses(ctx context.Context, courses []model.Course) {
	if Logger != nil {
		Logger(ctx, "Updating multiple courses.")
	}
	for i := range courses {
		dbErr := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&courses[i]).Error
		if dbErr != nil {
			if Logger != nil {
				Logger(ctx, fmt.Sprintf("Failed to save a course: %v\n", dbErr))
			}
		}
	}
}

func UpdateCourse(ctx context.Context, course model.Course) {
	dbErr := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&course).Error
	if dbErr != nil {
		if Logger != nil {
			Logger(ctx, fmt.Sprintf("Failed to save a course: %v\n", dbErr))
		}
	}
}

func CreateCourse(ctx context.Context, course model.Course) error {
	if Logger != nil {
		Logger(ctx, "Creating course.")
	}
	err := DB.Create(&course).Error
	return err
}
