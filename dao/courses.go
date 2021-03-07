package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"gorm.io/gorm/clause"
)

func GetCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	var foundCourses []model.Course
	dbErr := DB.Find(&foundCourses, "user_id = ?", userid).Error
	return foundCourses, dbErr
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
* Saves all provided courses into database. This might be faster with some sort of batch insert but
* we shouldn't have more than ~ 20 courses so it should be fine.
**/
func UpdateCourses(ctx context.Context, courses []model.Course) {
	if Logger != nil {
		Logger(ctx, "Updating multiple courses.")
	}
	for i := range courses {
		// We have to reimport all students because we can't get info on who left the course from tumonline
		dbErr := DB.Delete(&model.StudentToCourse{}, "course_id = ?", courses[i].ID).Error
		if dbErr != nil {
			if Logger != nil {
				Logger(ctx, fmt.Sprintf("Failed to remove students from course: %v\n", dbErr))
			}
		}
		// skip on same configuration of users->courses
		dbErr = DB.Clauses(clause.OnConflict{DoNothing: true}).Save(courses[i]).Error
		if dbErr != nil {
			if Logger != nil {
				Logger(ctx, fmt.Sprintf("Failed to save a course: %v\n", dbErr))
			}
		}
	}
}
