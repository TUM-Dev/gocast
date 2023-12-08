// Package services provides functions for fetching data from the database.
package services

import (
	"gorm.io/gorm"
	"errors"
	"github.com/TUM-Dev/gocast/model"
	"net/http"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
)

func GetStreamByID(db *gorm.DB, streamID uint) (*model.Stream, error) {
    stream := &model.Stream{}
    err := db.Where("streams.id = ?", streamID).First(stream).Error
    if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, err
    } else if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

    return stream, nil
}

func GetEnrolledOrPublicLiveStreams(db *gorm.DB, userID *uint) ([]*model.Stream, error) {
    var streams []*model.Stream

    err := db.Table("streams").
        Joins("join courses on streams.course_id = courses.id").
        Joins("left join course_users on courses.id = course_users.course_id").
        Where("(course_users.user_id = ? OR courses.visibility = \"public\") AND streams.live_now = true", *userID).
        Find(&streams).Error

    if err != nil {
        return nil, err
    }

    return streams, nil
}