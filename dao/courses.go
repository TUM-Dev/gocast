package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"time"
)

//GetCurrentOrNextLectureForCourse Gets the next lecture for a course or the lecture that is currently live. Error otherwise.
func GetCurrentOrNextLectureForCourse(ctx context.Context, courseID uint) (model.Stream, error) {
	var res model.Stream
	err := DB.Model(&model.Stream{}).Preload("Chats").Order("start").First(&res, "course_id = ? AND (end > NOW() OR live_now)", courseID).Error
	return res, err
}

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
		err = DB.Preload("Streams", "start BETWEEN DATESUB(month, 1, NOW()) and DATEADD(month, 3, NOW())").Find(&courses).Error
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
	var publicCourses []model.Course
	err = DB.Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.Order("start asc")
	}).Find(&publicCourses, "visibility = 'public' AND teaching_term = ? AND year = ?", term, year).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("publicCourses%v%v", year, term), publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func DeleteCourse(course model.Course) {
	for _, stream := range course.Streams {
		err := DB.Delete(&stream).Error
		if err != nil {
			log.WithError(err).Error("Can't delete stream")
		}
	}
	err := DB.Model(&course).Updates(map[string]interface{}{"live_enabled": false, "vod_enabled": false}).Error
	if err != nil {
		log.WithError(err).Error("Can't update course settings when deleting")
	}
	err = DB.Delete(&course).Error
	if err != nil {
		log.WithError(err).Error("Can't delete course")
	}
}

func GetCourseByToken(token string) (course model.Course, err error) {
	err = DB.Unscoped().
		Preload("Streams", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		First(&course, "token = ?", token).Error
	return
}

func GetCourseById(ctx context.Context, id uint) (course model.Course, err error) {
	var foundCourse model.Course
	dbErr := DB.Preload("Streams.Stats").Preload("Streams.Files").Preload("Streams", func(db *gorm.DB) *gorm.DB {
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
	var foundCourses []model.Course
	dbErr := DB.Where("tum_online_identifier <> '' AND year = ? AND teaching_term = ?", year, term).Find(&foundCourses).Error
	return foundCourses, dbErr
}

func UpdateCourseMetadata(ctx context.Context, course model.Course) {
	defer Cache.Clear()
	DB.Save(&course)
}

func UpdateCourseSettings(ctx context.Context, course model.Course) error {
	return DB.Model(&course).Updates(map[string]interface{}{
		"deleted_at":                course.DeletedAt,
		"visibility":                course.Visibility,
		"vod_enabled":               course.VODEnabled,
		"live_enabled":              course.LiveEnabled,
		"downloads_enabled":         course.DownloadsEnabled,
		"chat_enabled":              course.ChatEnabled,
		"vod_chat_enabled":          course.VodChatEnabled,
		"name":                      course.Name,
		"user_id":                   course.UserID,
		"user_created_by_token":     course.UserCreatedByToken,
		"camera_preset_preferences": course.CameraPresetPreferences,
	}).Error
}

func UpdateCourse(ctx context.Context, course model.Course) error {
	defer Cache.Clear()
	return DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&course).Error
}

func CreateCourse(ctx context.Context, course model.Course, keep bool) error {
	defer Cache.Clear()
	err := DB.Create(&course).Error
	if err != nil {
		return err
	}
	if !keep {
		err = DB.Model(&course).Updates(map[string]interface{}{"live_enabled": "0"}).Error
		if err != nil {
			log.WithError(err).Error("Can't update live enabled state")
		}
		return DB.Delete(&course).Error
	}
	return nil
}

func GetAvailableSemesters(c context.Context) []Semester {
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
