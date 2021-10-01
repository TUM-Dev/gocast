package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func FindPreset(lectureHallID string, presetID string) (model.CameraPreset, error) {
	var preset model.CameraPreset
	err := DB.First(&preset, "preset_id = ? AND lecture_hall_id = ?", presetID, lectureHallID).Error
	return preset, err
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

func HasStreamLectureHall(id uint) bool {
	var isNull bool
	DB.Raw("SELECT lecture_hall_id IS NULL FROM streams WHERE ID = ?", id).Scan(&isNull)
	return !isNull
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

func SaveLectureHallFullAssoc(lectureHall model.LectureHall) {
	DB.Clauses(clause.OnConflict{UpdateAll: true}).Session(&gorm.Session{FullSaveAssociations: true}).Updates(&lectureHall)
}

func SaveLectureHall(lectureHall model.LectureHall) {
	DB.Save(&lectureHall)
}

func UnsetLectureHall(lectureID uint) {
	DB.Model(&model.Stream{}).
		Where("id = ?", lectureID).
		Select("lecture_hall_id").
		Updates(map[string]interface{}{"lecture_hall_id": nil})
}
