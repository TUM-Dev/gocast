package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=texts.go -destination ../mock_dao/texts.go

type TextDao interface {
	New(*model.Text) error
	GetAll() ([]model.Text, error)
	Update(uint, *model.Text) error
}

type textDao struct {
	db *gorm.DB
}

func NewTextDao() TextDao {
	return textDao{db: DB}
}

func (d textDao) New(text *model.Text) error {
	return DB.Create(text).Error
}

func (d textDao) GetAll() (texts []model.Text, err error) {
	err = DB.Find(&texts).Error
	return texts, err
}

func (d textDao) Update(id uint, text *model.Text) error {
	return DB.Model(&model.Text{}).Where("id = ?", id).Updates(text).Error
}
