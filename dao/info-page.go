package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=infopage.go -destination ../mock_dao/infopage.go

type InfoPageDao interface {
	New(*model.InfoPage) error
	GetAll() ([]model.InfoPage, error)
	GetById(uint) (model.InfoPage, error)
	Update(uint, *model.InfoPage) error
}

type infoPageDao struct {
	db *gorm.DB
}

func NewInfoPageDao() InfoPageDao {
	return infoPageDao{db: DB}
}

func (d infoPageDao) New(page *model.InfoPage) error {
	return DB.Create(page).Error
}

func (d infoPageDao) GetAll() (pages []model.InfoPage, err error) {
	err = DB.Find(&pages).Error
	return pages, err
}

func (d infoPageDao) GetById(id uint) (page model.InfoPage, err error) {
	err = DB.Find(&page, "id = ?", id).Error
	return page, err
}

func (d infoPageDao) Update(id uint, page *model.InfoPage) error {
	return DB.Model(&model.InfoPage{}).Where("id = ?", id).Updates(page).Error
}
