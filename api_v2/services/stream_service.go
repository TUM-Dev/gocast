// Package services provides functions for fetching data from the database.
package services

import (
	"gorm.io/gorm"
	"errors"
	"github.com/TUM-Dev/gocast/model"
	"net/http"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
)

// GetStreamByID retrieves a stream by its id.
func GetStreamByID(db *gorm.DB, streamID uint) (*model.Stream, error) {
    s := &model.Stream{}
    err := db.Where("streams.id = ?", streamID).First(s).Error
    if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, err
    } else if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

    return s, nil
}

// GetStreamsByCourseID retrieves all streams of a course by its id.
func GetStreamsByCourseID(db *gorm.DB, courseID uint) ([]*model.Stream, error) {
    var streams []*model.Stream
    if err := db.Where("streams.course_id = ?", courseID).Find(&streams).Error; err != nil {
        return nil, err
	}

    return streams, nil
}

func GetEnrolledOrPublicLiveStreams(db *gorm.DB, uID *uint) ([]*model.Stream, error) {
    var streams []*model.Stream
    if *uID == 0 {
        err := db.Table("streams").
        Joins("join courses on streams.course_id = courses.id").
        Joins("left join course_users on courses.id = course_users.course_id").
        Where("(course_users.user_id = ? OR courses.visibility = \"public\") AND streams.live_now = 1", *uID).
        Find(&streams).Error
        if err != nil {
            return nil, err
        }
    } else {
        err := db.Table("streams").
        Select("DISTINCT streams.*").
        Joins("join courses on streams.course_id = courses.id").
        Joins("left join course_users on courses.id = course_users.course_id").
        Where("(course_users.user_id = ? OR courses.visibility = \"public\" OR courses.visibility = \"loggedin\") AND streams.live_now = 1", *uID).
        Find(&streams).Error
        if err != nil {
            return nil, err
        }
    }

    
    return streams, nil
}