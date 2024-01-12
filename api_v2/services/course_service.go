// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"net/http"
	"time"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// GetCourseById fetches a course from the database based on the provided id.
func GetCourseById(db *gorm.DB, id uint) (*model.Course, error) {
	c := &model.Course{}
	if err := db.Where("id = ?", id).First(c).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("course not found"))
	}

	return c, nil
}

func GetStreamById(db *gorm.DB, id uint) (*model.Stream, error) {
	s := &model.Stream{}
	if err := db.Where("id = ?", id).First(s).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}
	return s, nil
}

// FetchCourses fetches public courses from the database based on the provided request.
// It filters the courses by year, term, and query if they are specified in the request.
// It returns a slice of Course models or an error if one occurs.
func FetchCourses(db *gorm.DB, req *protobuf.GetPublicCoursesRequest, uID *uint) ([]model.Course, error) {
	query := db.Where("visibility = \"public\"")
	if *uID != 0 {
		query = query.Or("visibility = \"loggedin\"")
	}
	if req.Year != 0 {
		query = query.Where("year = ?", req.Year)
	}
	if req.Term != "" {
		query = query.Where("teaching_term = ?", req.Term)
	}
	if req.Limit > 0 {
		query = query.Limit(int(req.Limit))
	}
	if req.Skip >= 0 {
		query = query.Offset(int(req.Skip))
	}

	var courses []model.Course
	err := query.Find(&courses).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return courses, nil
}

// FetchSemesters fetches all unique semesters from the database.
// It returns a slice of Semester models or an error if one occurs.
func FetchSemesters(db *gorm.DB) ([]dao.Semester, error) {
	var semesters []dao.Semester
	err := db.Raw("SELECT year, teaching_term from courses " +
		"group by year, teaching_term " +
		"order by year desc, teaching_term desc").Scan(&semesters).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return semesters, nil
}

// FetchCurrentSemester determines the current semester based on the current date.
// It returns the current Semester model or an error if one occurs.
func FetchCurrentSemester(db *gorm.DB) (dao.Semester, error) {
	var curTerm string
	var curYear int
	if time.Now().Month() >= 4 && time.Now().Month() < 10 {
		curTerm = "S"
		curYear = time.Now().Year()
	} else {
		curTerm = "W"
		if time.Now().Month() >= 10 {
			curYear = time.Now().Year()
		} else {
			curYear = time.Now().Year() - 1
		}
	}
	return dao.Semester{
		Year:         curYear,
		TeachingTerm: curTerm,
	}, nil
}
