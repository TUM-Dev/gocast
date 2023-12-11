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
func CheckAuthorized(db *gorm.DB, userID uint, courseID uint) (error) {
    course, err := s.FindCourseById(db, courseID)
    if err != nil {
        return err
    }

    switch course.Visibility {
    case "public":
        return nil
    case "private":
        return e.WithStatus(http.StatusForbidden, errors.New("course is private"))
    case "hidden":
        return e.WithStatus(http.StatusForbidden, errors.New("course is hidden"))
    case "loggedin":
        if userID == 0 {
			return e.WithStatus(http.StatusForbidden, errors.New("course is only accessible by logged in users"))
		} else {
			return nil
		}
    case "enrolled":
        return checkUserEnrolled(db, userID, courseID)
    default:
        return e.WithStatus(http.StatusForbidden, errors.New("course is not accessible"))
    }
}

func checkUserEnrolled(db *gorm.DB, userID uint, courseID uint) (error) {
    if userID == 0 {
        return e.WithStatus(http.StatusForbidden, errors.New("course can only be accessed by enrolled users"))
    }

    var count int64
    if err := db.Table("course_users").Where("user_id = ? AND course_id = ?", userID, courseID).Count(&count).Error; err != nil {
        return e.WithStatus(http.StatusInternalServerError, err)
    }

    if count == 0 {
        return e.WithStatus(http.StatusForbidden, errors.New("user is not enrolled in this course and the course can only be accessed by enrolled users"))
    }
    return nil
}
