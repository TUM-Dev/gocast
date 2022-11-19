package dao

import (
	"gorm.io/gorm"
)

//go:generate mockgen -source=statistics.go -destination ../mock_dao/statistics.go

type StatisticsDao interface {
	GetCourseNumStudents(courseID uint) (int64, error)
}

type statisticsDao struct {
	db *gorm.DB
}

func NewStatisticsDao() StatisticsDao {
	return statisticsDao{db: DB}
}

// GetCourseNumStudents returns the number of students enrolled in the course
func (d statisticsDao) GetCourseNumStudents(courseID uint) (int64, error) {
	var res int64
	err := DB.Table("course_users").Where("course_id = ? OR ? = 0", courseID, courseID).Count(&res).Error
	return res, err
}
