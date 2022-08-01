package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=info-page.go -destination ../mock_dao/info-page.go

type InfoPageDao interface {
	New(ctx context.Context, infoPage *model.InfoPage) error
	GetAll(ctx context.Context) ([]model.InfoPage, error)
	GetById(ctx context.Context, id uint) (model.InfoPage, error)
	Update(ctx context.Context, id uint, infoPage *model.InfoPage) error
}

type infoPageDao struct {
	db *gorm.DB
}

func NewInfoPageDao() InfoPageDao {
	return infoPageDao{db: DB}
}

func (d infoPageDao) New(ctx context.Context, page *model.InfoPage) error {
	return DB.WithContext(ctx).Create(page).Error
}

func (d infoPageDao) GetAll(ctx context.Context) (pages []model.InfoPage, err error) {
	err = DB.WithContext(ctx).Find(&pages).Error
	return pages, err
}

func (d infoPageDao) GetById(ctx context.Context, id uint) (page model.InfoPage, err error) {
	err = DB.WithContext(ctx).Find(&page, "id = ?", id).Error
	return page, err
}

func (d infoPageDao) Update(ctx context.Context, id uint, page *model.InfoPage) error {
	return DB.WithContext(ctx).Model(&model.InfoPage{}).Where("id = ?", id).Updates(page).Error
}
