// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	)	

// FetchUserCourses fetches the courses for a user from the database.
// It filters the courses by year, term, query, limit, and skip if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchUserCourses(db *gorm.DB, userID uint, req *protobuf.GetUserCoursesRequest) (courses []model.Course, err error) {
	query := db.Where("user_id = ?", userID)
	if req.Year != 0 {
		query = query.Where("year = ?", req.Year)
	}
	if req.Term != "" {
		query = query.Where("teaching_term = ?", req.Term)
	}
	if req.Query != "" {
		query = query.Where("name LIKE ?", "%"+req.Query+"%")
	}
	if req.Limit >= 0 {
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
func FetchUserPinnedCourses(db *gorm.DB, user model.User, req *protobuf.GetUserPinnedRequest) (courses []model.Course,err error) {
	query := db.Model(&user).
		Joins("left join pinned_courses on pinned_courses.user_id = users.id").
		Joins("left join courses on pinned_courses.course_id = courses.id").
		Select("courses.*").
		Where("users.id = ?", user.ID)

	if req.Year != 0 {
		query = query.Where("courses.year = ?", req.Year)
	}
	if req.Term != "" {
		query = query.Where("courses.teaching_term = ?", req.Term)
	}
	if req.Limit >= 0 {
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
	err = db.Preload("Streams").Model(&model.Course{}).
		Joins("JOIN course_admins ON courses.id = course_admins.course_id").
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
