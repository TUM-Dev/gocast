package dao

import (
	"TUM-Live/model"
)

func GetAllLectureHalls() []model.LectureHall {
	var lectureHalls []model.LectureHall
	_ = DB.Find(&lectureHalls)
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

func SaveLectureHall(lectureHall model.LectureHall) {
	DB.Save(&lectureHall)
}

func UnsetLectureHall(lectureID uint) {
	DB.Model(&model.Stream{}).
		Where("id = ?", lectureID).
		Select("lecture_hall_id").
		Updates(map[string]interface{}{"lecture_hall_id": nil})
}
