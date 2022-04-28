package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func FindPreset(lectureHallID string, presetID string) (model.CameraPreset, error) {
	var preset model.CameraPreset
	err := DB.First(&preset, "preset_id = ? AND lecture_hall_id = ?", presetID, lectureHallID).Error
	return preset, err
}

// UnsetDefaults makes all camera presets not default
func UnsetDefaults(lectureHallID string) error {
	return DB.Model(&model.CameraPreset{}).Where("lecture_hall_id = ?", lectureHallID).Update("default", nil).Error
}

func SavePreset(preset model.CameraPreset) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Save(&preset).Error
}

func GetAllLectureHalls() []model.LectureHall {
	var lectureHalls []model.LectureHall
	_ = DB.Preload("CameraPresets").Find(&lectureHalls)
	return lectureHalls
}

func CreateLectureHall(lectureHall model.LectureHall) {
	DB.Create(&lectureHall)
}

func GetLectureHallByPartialName(name string) (model.LectureHall, error) {
	var res model.LectureHall
	err := DB.Where("full_name LIKE ?", "%"+name+"%").First(&res).Error
	return res, err
}

func GetLectureHallByID(id uint) (model.LectureHall, error) {
	var lectureHall model.LectureHall
	err := DB.Preload("CameraPresets").First(&lectureHall, id).Error
	return lectureHall, err
}

func DeleteLectureHall(id uint) error {
	err := DB.Delete(&model.LectureHall{}, id).Error
	if err != nil {
		return err
	}

	DB.Delete(model.CameraPreset{}, "lecture_hall_id = ?", id)
	DB.Exec("UPDATE streams SET lecture_hall_id = NULL WHERE lecture_hall_id = ?", id)
	return nil
}

func SaveLectureHallFullAssoc(lectureHall model.LectureHall) {
	DB.Delete(model.CameraPreset{}, "lecture_hall_id = ?", lectureHall.ID)
	DB.Clauses(clause.OnConflict{UpdateAll: true}).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&lectureHall)
}

func SaveLectureHall(lectureHall model.LectureHall) error {
	return DB.Save(&lectureHall).Error
}

// GetStreamsForLectureHallIcal returns an instance of []calendarResult for the ical export.
// if a user id is given, only streams of the user are returned. All streams are returned otherwise.
// streams that happened more than on month ago and streams that are more than 3 months in the future are omitted.
func GetStreamsForLectureHallIcal(userId uint) ([]CalendarResult, error) {
	var res []CalendarResult
	err := DB.Model(&model.Stream{}).
		Joins("LEFT JOIN lecture_halls ON lecture_halls.id = streams.lecture_hall_id").
		Joins("JOIN courses ON courses.id = streams.course_id").
		Select("streams.id as stream_id, streams.created_at as created, "+
			"lecture_halls.name as lecture_hall_name, "+
			"streams.start, streams.end, courses.name as course_name").
		Where("(streams.start BETWEEN DATE_SUB(NOW(), INTERVAL 1 MONTH) and DATE_ADD(NOW(), INTERVAL 3 MONTH)) "+
			"AND (courses.user_id = ? OR 0 = ?)", userId, userId).
		Scan(&res).Error
	return res, err
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
