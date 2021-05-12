package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetAllLectureHalls() []model.LectureHall {
	var lectureHalls []model.LectureHall
	_ = DB.Preload("CameraPresets").Find(&lectureHalls)
	return lectureHalls
}

func CreateLectureHall(lectureHall model.LectureHall) {
	DB.Create(&lectureHall)
}

func GetLectureHallByID(id uint) (model.LectureHall, error) {
	var lectureHall model.LectureHall
	err := DB.First(&lectureHall, id).Error
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
