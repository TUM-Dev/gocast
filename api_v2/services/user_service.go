package services

import (
	"errors"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	)	

	func RetrievePinnedCourses(db *gorm.DB, user model.User, req *protobuf.GetUserPinnedRequest) (courses []model.Course,err error) {
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

func RetrieveCourses(db *gorm.DB, userID uint, req *protobuf.GetUserCoursesRequest) (courses []model.Course, err error) {
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

func RetrieveUserAdminCourses(db *gorm.DB, userID uint) (courses []model.Course, err error) {
	err = db.Preload("Streams").Model(&model.Course{}).
		Joins("JOIN course_admins ON courses.id = course_admins.course_id").
		Where("course_admins.user_id = ?", userID).
		Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	
	return courses, err
}

func RetrieveBookmarks(db *gorm.DB, userID uint, req *protobuf.GetBookmarksRequest) (bookmarks []model.Bookmark, err error) {
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

func RetrieveUserSettings(db *gorm.DB, userID uint) (settings []model.UserSetting, err error) {
	err = db.Where("user_id = ?", userID).
		Find(&settings).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	
	return settings, err
}
