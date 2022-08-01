package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

//go:generate mockgen -source=lecture_halls.go -destination ../mock_dao/lecture_halls.go

type LectureHallsDao interface {
	CreateLectureHall(ctx context.Context, lectureHall model.LectureHall)
	SavePreset(ctx context.Context, preset model.CameraPreset) error
	SaveLectureHallFullAssoc(ctx context.Context, lectureHall model.LectureHall)
	SaveLectureHall(ctx context.Context, lectureHall model.LectureHall) error

	FindPreset(ctx context.Context, lectureHallID string, presetID string) (model.CameraPreset, error)
	GetAllLectureHalls(ctx context.Context) []model.LectureHall
	GetLectureHallByPartialName(ctx context.Context, name string) (model.LectureHall, error)
	GetLectureHallByID(ctx context.Context, id uint) (model.LectureHall, error)
	GetStreamsForLectureHallIcal(ctx context.Context, userId uint) ([]CalendarResult, error)

	UnsetDefaults(ctx context.Context, lectureHallID string) error

	DeleteLectureHall(ctx context.Context, id uint) error
}

type lectureHallsDao struct {
	db *gorm.DB
}

func NewLectureHallsDao() LectureHallsDao {
	return lectureHallsDao{db: DB}
}

func (d lectureHallsDao) CreateLectureHall(ctx context.Context, lectureHall model.LectureHall) {
	DB.WithContext(ctx).Create(&lectureHall)
}

func (d lectureHallsDao) SavePreset(ctx context.Context, preset model.CameraPreset) error {
	return DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Save(&preset).Error
}

func (d lectureHallsDao) SaveLectureHallFullAssoc(ctx context.Context, lectureHall model.LectureHall) {
	DB.WithContext(ctx).Delete(model.CameraPreset{}, "lecture_hall_id = ?", lectureHall.ID)
	DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&lectureHall)
}

func (d lectureHallsDao) SaveLectureHall(ctx context.Context, lectureHall model.LectureHall) error {
	return DB.WithContext(ctx).Save(&lectureHall).Error
}

func (d lectureHallsDao) FindPreset(ctx context.Context, lectureHallID string, presetID string) (model.CameraPreset, error) {
	var preset model.CameraPreset
	err := DB.WithContext(ctx).First(&preset, "preset_id = ? AND lecture_hall_id = ?", presetID, lectureHallID).Error
	return preset, err
}

func (d lectureHallsDao) GetAllLectureHalls(ctx context.Context) []model.LectureHall {
	var lectureHalls []model.LectureHall
	_ = DB.WithContext(ctx).Preload("CameraPresets").Find(&lectureHalls)
	return lectureHalls
}

func (d lectureHallsDao) GetLectureHallByPartialName(ctx context.Context, name string) (model.LectureHall, error) {
	var res model.LectureHall
	err := DB.WithContext(ctx).Where("full_name LIKE ?", "%"+name+"%").First(&res).Error
	return res, err
}

func (d lectureHallsDao) GetLectureHallByID(ctx context.Context, id uint) (model.LectureHall, error) {
	var lectureHall model.LectureHall
	err := DB.WithContext(ctx).Preload("CameraPresets").First(&lectureHall, id).Error
	return lectureHall, err
}

// GetStreamsForLectureHallIcal returns an instance of []calendarResult for the ical export.
// if a user id is given, only streams of the user are returned. All streams are returned otherwise.
// streams that happened more than on month ago and streams that are more than 3 months in the future are omitted.
func (d lectureHallsDao) GetStreamsForLectureHallIcal(ctx context.Context, userId uint) ([]CalendarResult, error) {
	var res []CalendarResult
	err := DB.WithContext(ctx).Model(&model.Stream{}).
		Joins("LEFT JOIN lecture_halls ON lecture_halls.id = streams.lecture_hall_id").
		Joins("JOIN courses ON courses.id = streams.course_id").
		Joins("JOIN course_admins ON courses.id = course_admins.course_id").
		Select("streams.id as stream_id, streams.created_at as created, "+
			"lecture_halls.name as lecture_hall_name, "+
			"streams.start, streams.end, courses.name as course_name").
		Where("(streams.start BETWEEN DATE_SUB(NOW(), INTERVAL 1 MONTH) and DATE_ADD(NOW(), INTERVAL 3 MONTH)) "+
			"AND (courses.user_id = ? OR 0 = ? OR course_admins.user_id = ?)", userId, userId, userId).
		Group("streams.id").
		Scan(&res).Error
	return res, err
}

// UnsetDefaults makes all camera presets not default
func (d lectureHallsDao) UnsetDefaults(ctx context.Context, lectureHallID string) error {
	return DB.WithContext(ctx).Model(&model.CameraPreset{}).Where("lecture_hall_id = ?", lectureHallID).Update("default", nil).Error
}

func (d lectureHallsDao) DeleteLectureHall(ctx context.Context, id uint) error {
	err := DB.WithContext(ctx).Delete(&model.LectureHall{}, id).Error
	if err != nil {
		return err
	}

	DB.WithContext(ctx).Delete(model.CameraPreset{}, "lecture_hall_id = ?", id)
	DB.WithContext(ctx).Exec("UPDATE streams SET lecture_hall_id = NULL WHERE lecture_hall_id = ?", id)
	return nil
}

type CalendarResult struct {
	StreamID        uint
	Created         time.Time
	Start           time.Time
	End             time.Time
	CourseName      string
	LectureHallName string
}

func (r CalendarResult) IsoStart() string {
	return r.Start.Format("20060102T150405")
}

func (r CalendarResult) IsoEnd() string {
	return r.End.Format("20060102T150405")
}

func (r CalendarResult) IsoCreated() string {
	return r.Created.Format("20060102T150405")
}
