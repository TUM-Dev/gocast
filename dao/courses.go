package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"
	"log"
	"time"
)

// GetAllCourses retrieves all courses from the database
// @limit bool true if streams should be limited to -1 month, +3 months
func GetAllCourses(limit bool) ([]model.Course, error) {
	cachedCourses, found := Cache.Get("allCourses")
	if found {
		return cachedCourses.([]model.Course), nil
	}
	var courses []model.Course
	var err error
	if !limit {
		err = DB.Preload("Streams").Find(&courses).Error
	} else {
		// limit 3 months in the future and one month in the past
		err = DB.Preload("Streams", "start BETWEEN ? and ?", time.Now().Add(time.Minute*60*24*30*-1), time.Now().Add(time.Minute*60*24*30*3)).Find(&courses).Error
	}
	if err == nil {
		Cache.SetWithTTL("allCourses", courses, 1, time.Minute)
	}
	return courses, err
}

func GetCourseForLecturerIdByYearAndTerm(c context.Context, year int, term string, userId uint) ([]model.Course, error) {
	var res []model.Course
	err := DB.Model(&model.Course{}).Find(&res, "user_id = ? AND year = ? AND teaching_term = ?", userId, year, term).Error
	return res, err
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

func GetCoursesForLoggedInUsers(year int, term string) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("loggedinCourses%v%v", year, term))
	if found {
		return cachedCourses.([]model.Course), err
	}
	var publicCourses []model.Course
	err = DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&publicCourses, "visibility = 'loggedin' AND teaching_term = ? AND year = ?", term, year).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("loggedinCourses%v%v", year, term), publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func GetAllCoursesForSemester(year int, term string, ctx context.Context) (courses []model.Course) {
	span := sentry.StartSpan(ctx, "SQL: GetAllCoursesForSemester")
	defer span.Finish()
	var foundCourses []model.Course
	DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&foundCourses, "teaching_term = ? AND year = ?", term, year)
	return foundCourses
}

func GetPublicCourses(year int, term string) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("publicCourses%v%v", year, term))
	if found {
		return cachedCourses.([]model.Course), err
	}
	log.Printf("not using cache!")
	var publicCourses []model.Course
	err = DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&publicCourses, "visibility = 'public' AND teaching_term = ? AND year = ?", term, year).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("publicCourses%v%v", year, term), publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func GetCourseById(ctx context.Context, id uint) (courses model.Course, err error) {
	var foundCourse model.Course
	dbErr := DB.Preload("Streams.Stats").Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("streams.start asc")
	}).Find(&foundCourse, "id = ?", id).Error
	return foundCourse, dbErr
}

func GetInvitedUsersForCourse(course *model.Course) error {
	return DB.Preload("Users", "role = ?", model.GenericType).Find(course).Error
}

func GetCourseBySlugYearAndTerm(ctx context.Context, slug string, term string, year string) (model.Course, error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("courseBySlugYearAndTerm%v%v%v", slug, term, year))
	if found {
		return cachedCourses.(model.Course), nil
	}
	var course model.Course
	err := DB.Preload("Streams.Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_start asc")
	}).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Where("teaching_term = ? AND slug = ? AND year = ?", term, slug, year).First(&course).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("courseBySlugYearAndTerm%v%v%v", slug, term, year), course, 1, time.Minute)
	}
	return course, err
}

func GetAllCoursesWithTUMIDForSemester(ctx context.Context, year int, term string) (courses []model.Course, err error) {
	if Logger != nil {
		Logger(ctx, "Find all courses with tum_online_identifier")
	}
	var foundCourses []model.Course
	dbErr := DB.Where("tum_online_identifier <> '' AND year = ? AND teaching_term = ?", year, term).Find(&foundCourses).Error
	if dbErr != nil {
		if Logger != nil {
			Logger(ctx, fmt.Sprintf("Unable to query courses with tum_online_identifier:%v\n", dbErr))
		}
		return nil, err
	}
	return foundCourses, nil
}

// UpdateCourses Saves all provided courses into database.
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

func UpdateCourseMetadata(ctx context.Context, course model.Course) {
	defer Cache.Clear()
	DB.Save(&course)
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

func GetAvailableSemesters(c context.Context) []Semester {
	span := sentry.StartSpan(c, "SQL: GetAvailableSemesters")
	defer span.Finish()
	if cached, found := Cache.Get("getAllSemesters"); found {
		return cached.([]Semester)
	} else {
		var semesters []Semester
		DB.Raw("SELECT year, teaching_term from courses " +
			"group by year, teaching_term " +
			"order by year desc, teaching_term desc").Scan(&semesters)
		Cache.SetWithTTL("getAllSemesters", semesters, 1, time.Hour)
		return semesters
	}
}

func IsUserAllowedToWatchPrivateCourse(course model.Course, user *model.User) bool {
	if user != nil {
		for _, c := range user.Courses {
			if c.ID == course.ID {
				return true
			}
		}
		return user.Role == model.AdminType || user.ID == course.UserID
	}
	return false
}

func GetCourseByShortLink(link string) (model.Course, error) {
	var courseId uint
	err := DB.Model(&model.ShortLink{}).Select("course_id").Where("link = ?", link).Scan(&courseId).Error
	if err != nil {
		return model.Course{}, err
	}
	course, err := GetCourseById(context.Background(), courseId)
	return course, err
}

type Semester struct {
	TeachingTerm string
	Year         int
}
