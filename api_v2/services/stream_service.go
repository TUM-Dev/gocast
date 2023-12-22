// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
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

// GetProgress retrieves the progress of a stream for a user.
func GetProgress(db *gorm.DB, streamID uint, userID uint) (*model.StreamProgress, error) {
	p := &model.StreamProgress{}

	err := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("progress not found"))
	}

	return p, nil
}

func SetProgress(db *gorm.DB, streamID uint, userID uint, progress float64) (*model.StreamProgress, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	if progress < 0 || progress > 1 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("progress must be between 0 and 1"))
	}

	p := &model.StreamProgress{}

	result := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		p.StreamID = streamID
		p.UserID = userID
		p.Progress = progress
		p.Watched = progress == 1
	} else if result.Error != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	} else {
		p.Progress = progress
		p.Watched = progress == 1
	}

	if err := db.Save(p).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return p, nil
}

func MarkAsWatched(db *gorm.DB, streamID uint, userID uint) (*model.StreamProgress, error) {

	_, err := GetStreamByID(db, streamID)

	if err != nil {
		return nil, err
	}

	p := &model.StreamProgress{}

	result := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		p.StreamID = streamID
		p.UserID = userID
		p.Progress = 1
		p.Watched = true
	} else if result.Error != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	} else {
		p.Progress = 1
		p.Watched = true
	}

	if err := db.Save(p).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return p, nil
}
