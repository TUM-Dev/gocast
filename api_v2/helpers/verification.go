// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

import (
	"gorm.io/gorm"
    "errors"
	"net/http"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
	s "github.com/TUM-Dev/gocast/api_v2/services"
)
// Veriification to check if user is authorized to access course/stream
func CheckEnrolledOrPublic(db *gorm.DB, userID *uint, courseID uint) (bool, error) {
    isPublic, err := checkPublic(db, courseID)
    if err != nil || isPublic {
        return isPublic, err
    }

	if userID == nil {
		return false, e.WithStatus(http.StatusForbidden, errors.New("course is not public"))
    }

    var count int64
    if err := db.Table("course_users").Where("user_id = ? AND course_id = ?", *userID, courseID).Count(&count).Error; err != nil {
        return false, e.WithStatus(http.StatusInternalServerError, err)
    }

    if count == 0 {
        return false, e.WithStatus(http.StatusForbidden, errors.New("user is not enrolled in this course and the course is not public"))
    }

    return true, nil
}

func checkPublic(db *gorm.DB, id uint) (bool, error) {
	if course, err := s.FindCourseById(db, id); err != nil {
		return false, err
	} else {
		if course.Visibility == "private" {
			return false, e.WithStatus(http.StatusForbidden, errors.New("course is private"))
			} else {
			return course.Visibility == "public", nil
		}
	}
}
