// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// FetchUserCourses fetches the courses for a user from the database.
// It filters the courses by year, term, query, limit, and skip if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchUserCourses(db *gorm.DB, userID uint, req *protobuf.GetUserCoursesRequest) (courses []model.Course, err error) {
	query := db.Unscoped().Table("course_users").
        Joins("join courses on course_users.course_id = courses.id").
        Select("courses.*").
        Where("course_users.user_id = ?", userID)

	if req.Year != 0 {
		query = query.Where("courses.year = ?", req.Year)
	}
	if req.Term != "" {
		query = query.Where("courses.teaching_term = ?", req.Term)
	}
	if req.Limit > 0 {
		query = query.Limit(int(req.Limit))
	}
	if req.Skip >= 0 {
		query = query.Offset(int(req.Skip))
	}

	err = query.Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return courses, nil
}

// FetchUserPinnedCourses fetches the pinned courses for a user from the database.
// It filters the courses by year, term, limit, and skip if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchUserPinnedCourses(db *gorm.DB, userID uint, req *protobuf.GetUserPinnedRequest) (courses []model.Course,err error) {
	query := db.Unscoped().Table("pinned_courses").
		Joins("join courses on pinned_courses.course_id = courses.id").
		Select("courses.*").
		Where("pinned_courses.user_id = ?", userID)

	if req.Year != 0 {
		query = query.Where("courses.year = ?", req.Year)
	}
	if req.Term != "" {
		query = query.Where("courses.teaching_term = ?", req.Term)
	}
	if req.Limit > 0 {
		query = query.Limit(int(req.Limit))
	}
	if req.Skip >= 0 {
		query = query.Offset(int(req.Skip))
	}

	err = query.Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return courses, nil
}

// FetchUserAdminCourses fetches the courses where a user is an admin from the database.
// It returns a slice of Course models or an error if one occurs.
func FetchUserAdminCourses(db *gorm.DB, userID uint) (courses []model.Course, err error) {
	err = db.Unscoped().Table("course_admins").
		Joins("JOIN courses ON course_admins.course_id = courses.id").
		Select("courses.*").
		Where("course_admins.user_id = ?", userID).
		Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	
	return courses, err
}

// FetchUserBookmarks fetches the bookmarks for a user from the database.
// It filters the bookmarks by stream ID if it is specified in the request.
// It returns a slice of Bookmark models or an error if one occurs.
func FetchUserBookmarks(db *gorm.DB, userID uint, req *protobuf.GetBookmarksRequest) (bookmarks []model.Bookmark, err error) {
	query := db.Where("user_id = ?", userID)

	if req.StreamID != 0 {
		query = query.Where("stream_id = ?", req.StreamID)
	}

	err = query.Find(&bookmarks).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return bookmarks, err
}

// FetchUserSettings fetches the settings for a user from the database.
// It returns a slice of UserSetting models or an error if one occurs.
func FetchUserSettings(db *gorm.DB, userID uint) (settings []model.UserSetting, err error) {
	err = db.Where("user_id = ?", userID).
		Find(&settings).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return settings, err
}

func PutUserBookmark(db *gorm.DB, userID uint, req *protobuf.PutBookmarkRequest) (bookmark *model.Bookmark, err error) {
	// check if bookmark already exists and if stream exists

	// first check if stream exists
	var stream model.Stream
	if err = db.Where("id = ?", req.StreamID).First(&stream).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

	bookmark = &model.Bookmark{
		Description: req.Description,
		Hours:       uint(req.Hours),
		Minutes:     uint(req.Minutes),
		Seconds:     uint(req.Seconds),
		UserID:      userID,
		StreamID:    uint(req.StreamID),
	}

	if err = db.Create(bookmark).Error; err != nil {
		return nil, err
	}

	return bookmark, nil
}

func PatchUserBookmark(db *gorm.DB, userID uint, req *protobuf.PatchBookmarkRequest) (bookmark *model.Bookmark, err error) {

	//	check if bookmark exists otherwise cannot patch
	if err = db.Where("id = ?", req.BookmarkID).First(&bookmark).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("bookmark not found"))
	}

	// check user allowed to patch bookmark
	if bookmark.UserID != userID {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("user not allowed to patch bookmark"))
	}

	//	patch it
	bookmark.Description = req.Description
	bookmark.Hours = uint(req.Hours)
	bookmark.Minutes = uint(req.Minutes)
	bookmark.Seconds = uint(req.Seconds)

	if err = db.Save(&bookmark).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} 

	return bookmark, nil
}

func DeleteUserBookmark(db *gorm.DB, userID uint, req *protobuf.DeleteBookmarkRequest) (err error) {

	//	check if bookmark exists otherwise cannot delete
	var bookmark model.Bookmark
	if err = db.Where("id = ?", req.BookmarkID).First(&bookmark).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusNotFound, errors.New("bookmark not found"))
	}

	// check user allowed to delete bookmark
	if bookmark.UserID != userID {
		return e.WithStatus(http.StatusUnauthorized, errors.New("user not allowed to delete bookmark"))
	}

	//	delete it
	if err = db.Delete(&bookmark).Error; err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}

func FetchBannerAlerts(db *gorm.DB) (alerts []model.ServerNotification, err error) {
	err = db.Where("start < now() AND expires > now()").Find(&alerts).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return alerts, err
}

// const (
// 	TargetAll      = iota + 1 //TargetAll Is any user, regardless if logged in or not
// 	TargetUser                //TargetUser Are all users that are logged in
// 	TargetStudent             //TargetStudent Are all users that are logged in and are students
// 	TargetLecturer            //TargetLecturer Are all users that are logged in and are lecturers
// 	TargetAdmin               //TargetAdmin Are all users that are logged in and are admins

// )

// 1 = admin
// 2 = Lecturer
// 3 = geneeric
// 4 = student

func getTargetFilter(user model.User) (targetFilter string) {
	switch user.Role {
	case 1:
		targetFilter = "target = 1"
	case 2:
		targetFilter = "target = 2"
	case 3:
		targetFilter = "target = 3"
	case 4:
		targetFilter = "target = 4"
	default:
		targetFilter = "target = 1"
	}
	return targetFilter
}

func FetchUserNotifications(db *gorm.DB, u *model.User) (notifications []model.Notification, err error) {
	targetFilter := getTargetFilter(*u)

	err = db.Where(targetFilter).Find(&notifications).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return notifications, nil
}
