package dao

import "github.com/joschahenningsen/TUM-Live/model"

func GetFileById(id string) (f model.File, err error) {
	err = DB.Where("id = ?", id).First(&f).Error
	return
}
