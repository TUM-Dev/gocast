package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"gorm.io/gorm"
	"time"
)

func GetCoursesByUserIdForTerm(ctx context.Context, userid uint, year int, term string) (courses []model.Course, err error) {
	c, e := GetCoursesByUserId(ctx, userid)
	if e != nil {
		return nil, err
	}
	var cRes []model.Course
	for _, cL := range c {
		if cL.Year == year && cL.TeachingTerm == term {
			cRes = append(cRes, cL)
		}
	}
	return cRes, nil
}

func GetCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("coursesByUserID%v", userid))
	if found {
		return cachedCourses.([]model.Course), nil
	}
	isAdmin, err := IsUserAdmin(ctx, userid)
	if err != nil {
		return nil, err
	}
	var foundCourses []model.Course
	if isAdmin {
		dbErr := DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
			return db.Order("start asc")
		}).Find(&foundCourses).Error
		if dbErr == nil {
			Cache.SetWithTTL(fmt.Sprintf("coursesByUserID%v", userid), foundCourses, 1, time.Minute)
		}
		return foundCourses, dbErr
	}
	dbErr := DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&foundCourses, "user_id = ?", userid).Error
	if dbErr == nil {
		Cache.SetWithTTL(fmt.Sprintf("coursesByUserID%v", userid), foundCourses, 1, time.Minute)
	}
	return foundCourses, dbErr
}

func GetPublicCourses(year int, term string) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("publicCourses%v%v", year, term))
	if found {
		return cachedCourses.([]model.Course), err
	}
	var publicCourses []model.Course
	err = DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&publicCourses, "visibility = 'public' AND teaching_term = ? AND year = ?", term, year).Error
	if err == nil {
		Cache.SetWithTTL("publicCourses", publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func GetCourseById(ctx context.Context, id uint) (courses model.Course, err error) {
	var foundCourse model.Course
	dbErr := DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&foundCourse, "id = ?", id).Error
	return foundCourse, dbErr
}

func GetCourseBySlugAndTerm(ctx context.Context, slug string, term string) (model.Course, error) {
	cachedCourses, found := Cache.Get("courseBySlugAndTerm")
	if found {
		return cachedCourses.(model.Course), nil
	}
	var course model.Course
	err := DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Where("teaching_term = ? AND slug = ?", term, slug).First(&course).Error
	if err == nil {
		Cache.SetWithTTL("courseBySlugAndTerm", course, 1, time.Minute)
	}
	return course, err
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
	defer Cache.Clear()
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
	defer Cache.Clear()
	dbErr := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&course).Error
	if dbErr != nil {
		if Logger != nil {
			Logger(ctx, fmt.Sprintf("Failed to save a course: %v\n", dbErr))
		}
	}
}

func CreateCourse(ctx context.Context, course model.Course) error {
	defer Cache.Clear()
	if Logger != nil {
		Logger(ctx, "Creating course.")
	}
	err := DB.Create(&course).Error
	return err
}
