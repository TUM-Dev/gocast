// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"net/http"
	"time"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// Helper method to fetch courses for a given table (e.g. course_users or pinned_courses)
func fetchCourses(db *gorm.DB, uID uint, year uint, term string, limit int, skip int, tableName string) (courses []model.Course, err error) {
	query := db.Unscoped().Table(tableName).
		Joins("join courses on "+tableName+".course_id = courses.id").
		Select("courses.*").
		Where(tableName+".user_id = ?", uID)

	if year != 0 {
		query = query.Where("courses.year = ?", year)
	}
	if term != "" {
		query = query.Where("courses.teaching_term = ?", term)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if skip >= 0 {
		query = query.Offset(skip)
	}

	err = query.Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return courses, nil
}

// FetchUserCourses fetches the courses for a user from the database.
// It filters the courses by year, term, query, limit, and skip if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchUserCourses(db *gorm.DB, uID uint, req *protobuf.GetUserCoursesRequest) (courses []model.Course, err error) {
	return fetchCourses(db, uID, uint(req.Year), req.Term, int(req.Limit), int(req.Skip), "course_users")
}

// FetchUserPinnedCourses fetches the pinned courses for a user from the database.
// It filters the courses by year, term, limit, and skip if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchUserPinnedCourses(db *gorm.DB, uID uint, req *protobuf.GetUserPinnedRequest) (courses []model.Course, err error) {
	return fetchCourses(db, uID, uint(req.Year), req.Term, int(req.Limit), int(req.Skip), "pinned_courses")
}

// FetchUserAdminCourses fetches the courses where a user is an admin from the database.
// It returns a slice of Course models or an error if one occurs.
func FetchUserAdminCourses(db *gorm.DB, uID uint) (courses []model.Course, err error) {
	err = db.Unscoped().Table("course_admins").
		Joins("JOIN courses ON course_admins.course_id = courses.id").
		Select("courses.*").
		Where("course_admins.user_id = ?", uID).
		Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return courses, err
}

// FetchUserBookmarks fetches the bookmarks for a user from the database.
// It filters the bookmarks by stream ID if it is specified in the request.
// It returns a slice of Bookmark models or an error if one occurs.
func FetchUserBookmarks(db *gorm.DB, uID uint, req *protobuf.GetBookmarksRequest) (bookmarks []model.Bookmark, err error) {
	query := db.Where("user_id = ?", uID)

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
func FetchUserSettings(db *gorm.DB, uID uint) (settings []model.UserSetting, err error) {
	err = db.Where("user_id = ?", uID).
		Find(&settings).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return settings, err
}

func PatchUserSettings(db *gorm.DB, user *model.User, req *protobuf.PatchUserSettingsRequest) (settings []model.UserSetting, err error) {
	userID := user.ID

	// value shouldn't be an empty string if name is changed
	for _, setting := range req.UserSettings {
		if setting.Type == *protobuf.UserSettingType_PREFERRED_NAME.Enum() {
			if setting.Value == "" {
				return nil, e.WithStatus(http.StatusBadRequest, errors.New("preferred name cannot be empty"))
			}
			// check if last name change is at least 3 months ago
			lastChange := model.UserSetting{}
			if err = db.Where("user_id = ? AND type = ?", userID, 1).First(&lastChange).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// no last change found, so we can just continue
			} else {
				diff := time.Now().Sub(lastChange.CreatedAt)
				if diff.Hours() < 24*30*3 {
					return nil, e.WithStatus(http.StatusBadRequest, errors.New("preferred name can only be changed every 3 months"))
				}
			}
		}
	}

	for _, setting := range req.UserSettings {
		var userSetting model.UserSetting
		if err = db.Where("user_id = ? AND type = ?", userID, setting.Type).First(&userSetting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			userSetting = model.UserSetting{
				UserID: userID,
				Type:   model.UserSettingType(setting.Type + 1),
				Value:  setting.Value,
			}
			if err = db.Create(&userSetting).Error; err != nil {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			}
		} else {
			userSetting.Value = setting.Value

			if err = db.Save(&userSetting).Error; err != nil {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			}
		}

		settings = append(settings, userSetting)
	}

	return settings, nil
}

func PutUserBookmark(db *gorm.DB, uID uint, req *protobuf.PutBookmarkRequest) (bookmark *model.Bookmark, err error) {
	// check if bookmark already exists and if stream exists

	// first check if stream exists
	var s model.Stream
	if err = db.Where("id = ?", req.StreamID).First(&s).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

	bookmark = &model.Bookmark{
		Description: req.Description,
		Hours:       uint(req.Hours),
		Minutes:     uint(req.Minutes),
		Seconds:     uint(req.Seconds),
		UserID:      uID,
		StreamID:    uint(req.StreamID),
	}

	if err = db.Create(bookmark).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return bookmark, nil
}

func PatchUserBookmark(db *gorm.DB, uID uint, req *protobuf.PatchBookmarkRequest) (bookmark *model.Bookmark, err error) {
	//	check if bookmark exists otherwise cannot patch
	if err = db.Where("id = ?", req.BookmarkID).First(&bookmark).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("bookmark not found"))
	}

	// check user allowed to patch bookmark
	if bookmark.UserID != uID {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("user not allowed to patch bookmark"))
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

func DeleteUserBookmark(db *gorm.DB, uID uint, req *protobuf.DeleteBookmarkRequest) (err error) {
	//	check if bookmark exists otherwise cannot delete
	var bookmark model.Bookmark
	if err = db.Where("id = ?", req.BookmarkID).First(&bookmark).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusNotFound, errors.New("bookmark not found"))
	}

	// check user allowed to delete bookmark
	if bookmark.UserID != uID {
		return e.WithStatus(http.StatusForbidden, errors.New("user not allowed to delete bookmark"))
	}

	//	delete it
	if err = db.Delete(&bookmark).Error; err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}

func DeleteUserPinned(db *gorm.DB, u *model.User, courseID uint) (err error) {
	// Check if user has course pinned
	if pinned, err := checkPinnedByID(db, u.ID, courseID); err != nil {
		return err
	} else if !pinned {
		return e.WithStatus(http.StatusNotFound, errors.New("course not pinned"))
	}

	// Check if course exists otherwise cannot delete
	course, err := GetCourseById(db, courseID)
	if err != nil {
		return err
	}

	// Pin course
	if err = pinCourse(db, false, u, course); err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}

func PostUserPinned(db *gorm.DB, u *model.User, c *model.Course) (err error) {
	// Check if user has course already pinned
	if pinned, err := checkPinnedByID(db, u.ID, c.ID); err != nil {
		return err
	} else if pinned {
		return e.WithStatus(http.StatusConflict, errors.New("course already pinned"))
	}

	// Pin course
	if err = pinCourse(db, true, u, c); err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}

// PRIVATE HELPER METHODS

// FindPinnedByID fetches a pinned course entry from the database based on the provided userID and courseID	.
func checkPinnedByID(db *gorm.DB, uID uint, courseID uint) (bool, error) {
	var result struct{}
	if err := db.Table("pinned_courses").Where("user_id = ? AND course_id = ?", uID, courseID).Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, e.WithStatus(http.StatusInternalServerError, err)
	}
	return true, nil
}

func pinCourse(db *gorm.DB, pin bool, u *model.User, c *model.Course) error {
	if pin {
		return db.Model(u).Association("PinnedCourses").Append(c)
	}
	return db.Model(u).Association("PinnedCourses").Delete(c)
}
