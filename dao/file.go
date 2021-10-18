package dao

import "TUM-Live/model"

func GetFileById(id string) (f model.File, err error) {
	err = DB.Where("id = ?", id).First(&f).Error
	return
}
