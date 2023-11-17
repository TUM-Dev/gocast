package api_v2

import (
	"context"
	"errors"
	"github.com/TUM-Dev/gocast/api_v2/e"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
	"net/http"
)

func (a *API) GetCourses(context.Context, *protobuf.GetCoursesRequest) (*protobuf.GetCoursesResponse, error) {
	var courses []model.Course
	err := a.db.Where("visibility = ?", "public").Find(&courses).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

	for i, course := range courses {
		resp[i] = &protobuf.Course{
			Id:           uint64(course.ID),
			Name:         course.Name,
			TeachingTerm: course.TeachingTerm,
			Year:         uint32(course.Year),
		}
	}

	return &protobuf.GetCoursesResponse{
		Courses: resp,
	}, nil
}
