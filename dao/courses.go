package dao

import (
	"context"
	"errors"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"time"

	"github.com/RBG-TUM/commons"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:generate mockgen -source=courses.go -destination ../mock_dao/courses.go

type CoursesDao interface {
	CreateCourse(ctx context.Context, course *model.Course, keep bool) error
	AddAdminToCourse(ctx context.Context, userID uint, courseID uint) error

	GetCurrentOrNextLectureForCourse(ctx context.Context, courseID uint) (model.Stream, error)
	GetAllCourses(ctx context.Context) ([]model.Course, error)
	GetCourseForLecturerIdByYearAndTerm(ctx context.Context, year int, term string, userId uint) ([]model.Course, error)
	GetAdministeredCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error)
	GetAllCoursesForSemester(ctx context.Context, year int, term string) (courses []model.Course)
	GetPublicCourses(ctx context.Context, year int, term string) (courses []model.Course, err error)
	GetPublicAndLoggedInCourses(ctx context.Context, year int, term string) (courses []model.Course, err error)
	GetCourseByToken(ctx context.Context, token string) (course model.Course, err error)
	GetCourseById(ctx context.Context, id uint) (course model.Course, err error)
	GetInvitedUsersForCourse(ctx context.Context, course *model.Course) error
	GetCourseBySlugYearAndTerm(ctx context.Context, slug string, term string, year int) (model.Course, error)
	GetAllCoursesWithTUMIDForSemester(ctx context.Context, year int, term string) (courses []model.Course, err error)
	GetAvailableSemesters(ctx context.Context) []Semester
	GetCourseByShortLink(ctx context.Context, link string) (model.Course, error)
	GetCourseAdmins(ctx context.Context, courseID uint) ([]model.User, error)

	UpdateCourse(ctx context.Context, course model.Course) error
	UpdateCourseMetadata(ctx context.Context, course model.Course)
	UnDeleteCourse(ctx context.Context, course model.Course) error

	RemoveAdminFromCourse(ctx context.Context, userID uint, courseID uint) error
	DeleteCourse(ctx context.Context, course model.Course)
}

type coursesDao struct {
	db       *gorm.DB
	usersDao UsersDao
}

func NewCoursesDao() coursesDao {
	return coursesDao{db: DB, usersDao: NewUsersDao()}
}

// CreateCourse creates a new course, if keep is false, deleted_at is set to NOW(),
// letting the user manually create the course again (opt-in)
func (d coursesDao) CreateCourse(ctx context.Context, course *model.Course, keep bool) error {
	defer Cache.Clear()
	err := DB.WithContext(ctx).Create(&course).Error
	if err != nil {
		return err
	}
	if !keep {
		err = DB.WithContext(ctx).Model(&course).Updates(map[string]interface{}{"live_enabled": "0"}).Error
		if err != nil {
			log.WithError(err).Error("Can't update live enabled state")
		}
		return DB.WithContext(ctx).Delete(&course).Error
	}
	return nil
}

func (d coursesDao) AddAdminToCourse(ctx context.Context, userID uint, courseID uint) error {
	defer Cache.Clear()
	return DB.WithContext(ctx).Exec("insert into course_admins (user_id, course_id) values (?, ?)", userID, courseID).Error
}

//GetCurrentOrNextLectureForCourse Gets the next lecture for a course or the lecture that is currently live. Error otherwise.
func (d coursesDao) GetCurrentOrNextLectureForCourse(ctx context.Context, courseID uint) (model.Stream, error) {
	var res model.Stream
	err := DB.WithContext(ctx).Model(&model.Stream{}).Preload("Chats").Order("start").First(&res, "course_id = ? AND (end > NOW() OR live_now)", courseID).Error
	return res, err
}

// GetAllCourses retrieves all courses from the database
func (d coursesDao) GetAllCourses(ctx context.Context) ([]model.Course, error) {
	cachedCourses, found := Cache.Get("allCourses")
	if found {
		return cachedCourses.([]model.Course), nil
	}
	var courses []model.Course
	err := DB.WithContext(ctx).Preload("Streams").Find(&courses).Error
	if err == nil {
		Cache.SetWithTTL("allCourses", courses, 1, time.Minute)
	}
	return courses, err
}

func (d coursesDao) GetCourseForLecturerIdByYearAndTerm(ctx context.Context, year int, term string, userId uint) ([]model.Course, error) {
	var res []model.Course
	err := DB.WithContext(ctx).Model(&model.Course{}).Find(&res, "user_id = ? AND year = ? AND teaching_term = ?", userId, year, term).Error
	return res, err
}

func (d coursesDao) GetAdministeredCoursesByUserId(ctx context.Context, userid uint) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("coursesByUserID%v", userid))
	if found {
		return cachedCourses.([]model.Course), nil
	}
	isAdmin, err := d.usersDao.IsUserAdmin(ctx, userid)
	if err != nil {
		return nil, err
	}
	var foundCourses []model.Course
	// all courses for admins
	if isAdmin {
		dbErr := DB.WithContext(ctx).Preload("Streams", func(db *gorm.DB) *gorm.DB {
			return db.WithContext(ctx).Order("start asc")
		}).Find(&foundCourses).Error
		if dbErr == nil {
			Cache.SetWithTTL(fmt.Sprintf("coursesByUserID%v", userid), foundCourses, 1, time.Minute)
		}
		return foundCourses, dbErr
	}
	dbErr := DB.WithContext(ctx).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("start asc")
	}).Find(&foundCourses, "user_id = ?", userid).Error

	if err != nil && errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var administeredCourses []model.Course
	err = DB.WithContext(ctx).Model(&model.Course{}).
		Joins("JOIN course_admins ON courses.id = course_admins.course_id").
		Where("course_admins.user_id = ? AND courses.deleted_at IS NULL", userid).
		Find(&administeredCourses).Error
	if err != nil {
		return nil, err
	}
	foundCourses = append(foundCourses, administeredCourses...)
	foundCourses = commons.Unique(foundCourses, func(c model.Course) uint { return c.ID })
	if dbErr == nil {
		Cache.SetWithTTL(fmt.Sprintf("coursesByUserID%v", userid), foundCourses, 1, time.Minute)
	}
	return foundCourses, dbErr
}

func (d coursesDao) GetAllCoursesForSemester(ctx context.Context, year int, term string) (courses []model.Course) {
	var foundCourses []model.Course
	DB.WithContext(ctx).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("start asc")
	}).Find(&foundCourses, "teaching_term = ? AND year = ?", term, year)
	return foundCourses
}

func (d coursesDao) GetPublicCourses(ctx context.Context, year int, term string) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("publicCourses%d%v", year, term))
	if found {
		return cachedCourses.([]model.Course), err
	}
	var publicCourses []model.Course

	err = DB.WithContext(ctx).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("start asc")
	}).Find(&publicCourses, "visibility = 'public' AND teaching_term = ? AND year = ?",
		term, year).Error

	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("publicCourses%d%v", year, term), publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func (d coursesDao) GetPublicAndLoggedInCourses(ctx context.Context, year int, term string) (courses []model.Course, err error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("publicAndLoggedInCourses%d%v", year, term))
	if found {
		return cachedCourses.([]model.Course), err
	}
	var publicCourses []model.Course

	err = DB.WithContext(ctx).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("start asc")
	}).Find(&publicCourses,
		"(visibility = 'public' OR visibility = 'loggedin') AND teaching_term = ? AND year = ?", term, year).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("publicAndLoggedInCourses%d%v", year, term), publicCourses, 1, time.Minute)
	}
	return publicCourses, err
}

func (d coursesDao) GetCourseByToken(ctx context.Context, token string) (course model.Course, err error) {
	err = DB.WithContext(ctx).Unscoped().
		Preload("Streams", func(db *gorm.DB) *gorm.DB { return db.WithContext(ctx).Unscoped() }).
		First(&course, "token = ?", token).Error
	return
}

func (d coursesDao) GetCourseById(ctx context.Context, id uint) (course model.Course, err error) {
	var foundCourse model.Course
	dbErr := DB.WithContext(ctx).Preload("Streams.TranscodingProgresses").Preload("Streams.Stats").Preload("Streams.Files").Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("streams.start desc")
	}).Find(&foundCourse, "id = ?", id).Error
	return foundCourse, dbErr
}

func (d coursesDao) GetInvitedUsersForCourse(ctx context.Context, course *model.Course) error {
	return DB.WithContext(ctx).Preload("Users", "role = ?", model.GenericType).Find(course).Error
}

func (d coursesDao) GetCourseBySlugYearAndTerm(ctx context.Context, slug string, term string, year int) (model.Course, error) {
	cachedCourses, found := Cache.Get(fmt.Sprintf("courseBySlugYearAndTerm%v%v%v", slug, term, year))
	if found {
		return cachedCourses.(model.Course), nil
	}
	var course model.Course
	err := DB.WithContext(ctx).Preload("Streams.Units", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("unit_start desc")
	}).Preload("Streams", func(db *gorm.DB) *gorm.DB {
		return db.WithContext(ctx).Order("start desc")
	}).Where("teaching_term = ? AND slug = ? AND year = ?", term, slug, year).First(&course).Error
	if err == nil {
		Cache.SetWithTTL(fmt.Sprintf("courseBySlugYearAndTerm%v%v%v", slug, term, year), course, 1, time.Minute)
	}
	return course, err
}

func (d coursesDao) GetAllCoursesWithTUMIDForSemester(ctx context.Context, year int, term string) (courses []model.Course, err error) {
	var foundCourses []model.Course
	dbErr := DB.WithContext(ctx).Where("tum_online_identifier <> '' AND year = ? AND teaching_term = ?", year, term).Find(&foundCourses).Error
	return foundCourses, dbErr
}

func (d coursesDao) GetAvailableSemesters(ctx context.Context) []Semester {
	if cached, found := Cache.Get("getAllSemesters"); found {
		return cached.([]Semester)
	} else {
		var semesters []Semester
		DB.WithContext(ctx).Raw("SELECT year, teaching_term from courses " +
			"group by year, teaching_term " +
			"order by year desc, teaching_term desc").Scan(&semesters)
		Cache.SetWithTTL("getAllSemesters", semesters, 1, time.Hour)
		return semesters
	}
}

// GetCourseByShortLink returns the course associated with the given short link (e.g. EIDI2022)
func (d coursesDao) GetCourseByShortLink(ctx context.Context, link string) (model.Course, error) {
	var sl model.ShortLink
	err := DB.WithContext(ctx).First(&sl, "link = ?", link).Error
	if err != nil {
		return model.Course{}, err
	}
	course, err := d.GetCourseById(context.Background(), sl.CourseId)
	return course, err
}

// GetCourseAdmins returns the admins of the given course excluding the creator (usually system) and the tumlive admins
func (d coursesDao) GetCourseAdmins(ctx context.Context, courseID uint) ([]model.User, error) {
	var admins []model.User
	err := DB.WithContext(ctx).Raw("select u.* from courses "+
		"join course_admins ca on courses.id = ca.course_id "+
		"join users u on u.id = ca.user_id "+
		"where course_id = ?", courseID).
		Scan(&admins).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return admins, nil
	}
	return admins, err
}

func (d coursesDao) UpdateCourse(ctx context.Context, course model.Course) error {
	defer Cache.Clear()
	return DB.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&course).Error
}

func (d coursesDao) UpdateCourseMetadata(ctx context.Context, course model.Course) {
	defer Cache.Clear()
	DB.WithContext(ctx).Save(&course)
}

func (d coursesDao) UnDeleteCourse(ctx context.Context, course model.Course) error {
	return DB.WithContext(ctx).Exec("UPDATE courses SET deleted_at = NULL WHERE id = ?", course.ID).Error
}

func (d coursesDao) RemoveAdminFromCourse(ctx context.Context, userID uint, courseID uint) error {
	defer Cache.Clear()
	return DB.WithContext(ctx).Exec("delete from course_admins where user_id = ? and course_id = ?", userID, courseID).Error
}

func (d coursesDao) DeleteCourse(ctx context.Context, course model.Course) {
	for _, stream := range course.Streams {
		err := DB.WithContext(ctx).Delete(&stream).Error
		if err != nil {
			log.WithError(err).Error("Can't delete stream")
		}
	}
	err := DB.WithContext(ctx).Model(&course).Updates(map[string]interface{}{"live_enabled": false, "vod_enabled": false}).Error
	if err != nil {
		log.WithError(err).Error("Can't update course settings when deleting")
	}
	err = DB.WithContext(ctx).Delete(&course).Error
	if err != nil {
		log.WithError(err).Error("Can't delete course")
	}
}

type Semester struct {
	TeachingTerm string
	Year         int
}
