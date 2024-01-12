// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	s "github.com/TUM-Dev/gocast/api_v2/services"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// Verification to check if user is authorized to access course/stream
func CheckAuthorized(db *gorm.DB, uID uint, courseID uint) (*model.Course, error) {
	c, err := s.GetCourseById(db, courseID)
	if err != nil {
		return nil, err
	}

	switch c.Visibility {
	case "public":
		return c, nil
	case "private":
		return nil, e.WithStatus(http.StatusForbidden, errors.New("course is private"))
	case "hidden":
		return nil, e.WithStatus(http.StatusForbidden, errors.New("course is hidden"))
	case "loggedin":
		if uID == 0 {
			return nil, e.WithStatus(http.StatusForbidden, errors.New("course is only accessible by logged in users"))
		} else {
			return c, nil
		}
	case "enrolled":
		return checkUserEnrolled(db, uID, c)
	default:
		return nil, e.WithStatus(http.StatusForbidden, errors.New("course is not accessible"))
	}
}

func CheckCanChat(db *gorm.DB, uID uint, streamID uint) (*model.Stream, error) {
	// in the future, we can add a check for the user's role in the course

	stream, err := s.GetStreamById(db, streamID)
	if err != nil {
		return nil, err
	}

	if !stream.ChatEnabled {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("chat is disabled for this stream"))
	}

	course, err := s.GetCourseById(db, stream.CourseID)
	if err != nil {
		return nil, err
	}

	if !stream.LiveNow {
		if (!course.ChatEnabled && !course.ModeratedChatEnabled) || !course.VodChatEnabled {
			return nil, e.WithStatus(http.StatusForbidden, errors.New("chat is disabled for this stream"))
		}
	} else {
		if !course.ChatEnabled && !course.ModeratedChatEnabled {
			return nil, e.WithStatus(http.StatusForbidden, errors.New("chat is disabled for this stream"))
		}
	}

	return stream, nil
}

func checkUserEnrolled(db *gorm.DB, uID uint, c *model.Course) (*model.Course, error) {
	if uID == 0 {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("course can only be accessed by enrolled users"))
	}

	var count int64
	if err := db.Table("course_users").Where("user_id = ? AND course_id = ?", uID, c.ID).Count(&count).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if count == 0 {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("user is not enrolled in this course and the course can only be accessed by enrolled users"))
	}
	return c, nil
}
