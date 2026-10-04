package dao

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=lecture_halls.go -destination ../mock_dao/lecture_halls.go

type LectureHallsDao interface {
	CreateLectureHall(lectureHall *model.LectureHall) error
	SavePreset(preset model.CameraPreset) error
	SaveLectureHallFullAssoc(lectureHall model.LectureHall)
	SaveLectureHall(lectureHall model.LectureHall) error

	FindPreset(lectureHallID string, presetID string) (model.CameraPreset, error)
	GetAllLectureHalls() []model.LectureHall
	GetLectureHallByPartialName(name string) (model.LectureHall, error)
	GetLectureHallByID(id uint) (model.LectureHall, error)
	GetStreamsForLectureHallIcal(userId uint, lectureHalls []uint, all bool) ([]CalendarResult, error)
	// GetSchedule returns the lectures overlapping [from, to) of the courses userID
	// owns or administers -- every course for userID 0 -- in the given lecture halls,
	// where hall 0 stands for lectures with none, or in every hall when all is set.
	GetSchedule(userID uint, from, to time.Time, lectureHalls []uint, all bool) ([]ScheduleEntry, error)

	UnsetDefaults(lectureHallID string) error

	DeleteLectureHall(id uint) error

	GetLiveStateForPwrCtrl() ([]PwrCtrlLiveState, error)
}

type PwrCtrlLiveState struct {
	PwrCtrlIP string
	NumLive   uint
}

type lectureHallsDao struct {
	db *gorm.DB
}

func NewLectureHallsDao() LectureHallsDao {
	return lectureHallsDao{db: DB}
}

func (d lectureHallsDao) GetLiveStateForPwrCtrl() ([]PwrCtrlLiveState, error) {
	var result []PwrCtrlLiveState

	err := DB.Raw(`select lh.pwr_ctrl_ip, (select count(*) from tumlive.streams s where s.lecture_hall_id=lh.id and s.live_now and s.deleted_at is NULL) num_live from tumlive.lecture_halls lh
	where lh.pwr_ctrl_ip is not NULL and lh.pwr_ctrl_ip != '' and lh.deleted_at is NULL`).Scan(&result).Error
	return result, err
}

func (d lectureHallsDao) CreateLectureHall(lectureHall *model.LectureHall) error {
	return DB.Create(lectureHall).Error
}

func (d lectureHallsDao) SavePreset(preset model.CameraPreset) error {
	return DB.Clauses(clause.OnConflict{UpdateAll: true}).Save(&preset).Error
}

func (d lectureHallsDao) SaveLectureHallFullAssoc(lectureHall model.LectureHall) {
	DB.Delete(model.CameraPreset{}, "lecture_hall_id = ?", lectureHall.ID)
	DB.Clauses(clause.OnConflict{UpdateAll: true}).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&lectureHall)
}

func (d lectureHallsDao) SaveLectureHall(lectureHall model.LectureHall) error {
	return DB.Save(&lectureHall).Error
}

func (d lectureHallsDao) FindPreset(lectureHallID string, presetID string) (model.CameraPreset, error) {
	var preset model.CameraPreset
	err := DB.First(&preset, "preset_id = ? AND lecture_hall_id = ?", presetID, lectureHallID).Error
	return preset, err
}

func (d lectureHallsDao) GetAllLectureHalls() []model.LectureHall {
	var lectureHalls []model.LectureHall
	_ = DB.Preload("CameraPresets").Find(&lectureHalls)
	return lectureHalls
}

func (d lectureHallsDao) GetLectureHallByPartialName(name string) (model.LectureHall, error) {
	var res model.LectureHall
	err := DB.Where("full_name LIKE ?", "%"+name+"%").First(&res).Error
	return res, err
}

func (d lectureHallsDao) GetLectureHallByID(id uint) (model.LectureHall, error) {
	var lectureHall model.LectureHall
	err := DB.Preload("CameraPresets").First(&lectureHall, id).Error
	return lectureHall, err
}

// GetStreamsForLectureHallIcal returns an instance of []calendarResult for the ical export.
// if a user id is given, only streams of the user are returned. All streams are returned otherwise.
// streams that happened more than on month ago and streams that are more than 3 months in the future are omitted.
func (d lectureHallsDao) GetStreamsForLectureHallIcal(userId uint, lectureHalls []uint, all bool) ([]CalendarResult, error) {
	var res []CalendarResult
	err := DB.Model(&model.Stream{}).
		Joins("LEFT JOIN lecture_halls ON lecture_halls.id = streams.lecture_hall_id").
		Joins("JOIN courses ON courses.id = streams.course_id").
		Joins("LEFT JOIN course_admins ON courses.id = course_admins.course_id").
		Select("streams.id as stream_id, streams.created_at as created, "+
			"lecture_halls.name as lecture_hall_name, "+
			"streams.start, streams.end, courses.name as course_name").
		Where("(streams.start BETWEEN DATE_SUB(NOW(), INTERVAL 1 MONTH) and DATE_ADD(NOW(), INTERVAL 3 MONTH)) "+
			"AND (courses.user_id = ? OR 0 = ? OR course_admins.user_id = ?) AND courses.deleted_at IS NULL "+
			"AND (streams.lecture_hall_id IN ? OR (0 in ? AND streams.lecture_hall_id is null) OR ?)", userId, userId, userId, lectureHalls, lectureHalls, all).
		Group("streams.id").
		Scan(&res).Error
	return res, err
}

// UnsetDefaults makes all camera presets not default
func (d lectureHallsDao) UnsetDefaults(lectureHallID string) error {
	return DB.Model(&model.CameraPreset{}).Where("lecture_hall_id = ?", lectureHallID).Update("default", nil).Error
}

func (d lectureHallsDao) DeleteLectureHall(id uint) error {
	err := DB.Delete(&model.LectureHall{}, id).Error
	if err != nil {
		return err
	}

	DB.Delete(model.CameraPreset{}, "lecture_hall_id = ?", id)
	DB.Exec("UPDATE streams SET lecture_hall_id = NULL WHERE lecture_hall_id = ?", id)
	return nil
}

// ScheduleEntry is one lecture on the administration schedule.
type ScheduleEntry struct {
	StreamID        uint
	CourseID        uint
	CourseName      string
	Name            string
	Description     string
	Start           time.Time
	End             time.Time
	LectureHallID   uint
	LectureHallName string
}

// GetSchedule is GetStreamsForLectureHallIcal for a window the caller chooses rather
// than a fixed one around today, with what the schedule page shows of each lecture.
// The administrator check is a subquery rather than v1's join, which repeated a
// lecture once per course admin and needed a GROUP BY to undo it.
func (d lectureHallsDao) GetSchedule(userID uint, from, to time.Time, lectureHalls []uint, all bool) ([]ScheduleEntry, error) {
	query := DB.Model(&model.Stream{}).
		Joins("LEFT JOIN lecture_halls ON lecture_halls.id = streams.lecture_hall_id").
		Joins("JOIN courses ON courses.id = streams.course_id").
		Select("streams.id AS stream_id, courses.id AS course_id, courses.name AS course_name, "+
			"streams.name, streams.description, streams.start, streams.end, "+
			"IFNULL(streams.lecture_hall_id, 0) AS lecture_hall_id, "+
			"IFNULL(lecture_halls.name, '') AS lecture_hall_name").
		Where("streams.start < ? AND streams.end > ?", to, from).
		Where("courses.deleted_at IS NULL").
		Where("(? = 0 OR courses.user_id = ? OR EXISTS "+
			"(SELECT 1 FROM course_admins WHERE course_admins.course_id = courses.id AND course_admins.user_id = ?))",
			userID, userID, userID).
		Order("streams.start")

	if !all {
		var halls []uint
		withoutHall := false
		for _, id := range lectureHalls {
			if id == 0 {
				withoutHall = true
			} else {
				halls = append(halls, id)
			}
		}

		// A lecture with no hall has lecture_hall_id NULL or 0, depending on how it
		// was saved.
		switch {
		case len(halls) > 0 && withoutHall:
			query = query.Where("(streams.lecture_hall_id IN ? OR streams.lecture_hall_id IS NULL OR streams.lecture_hall_id = 0)", halls)
		case len(halls) > 0:
			query = query.Where("streams.lecture_hall_id IN ?", halls)
		case withoutHall:
			query = query.Where("(streams.lecture_hall_id IS NULL OR streams.lecture_hall_id = 0)")
		default:
			return []ScheduleEntry{}, nil
		}
	}

	var res []ScheduleEntry
	err := query.Scan(&res).Error
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
