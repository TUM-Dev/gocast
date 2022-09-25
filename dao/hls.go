package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=hls.go -destination ../mock_dao/hls.go

type HlsDao interface {
	// Create a new Hls for the database
	Create(context.Context, *model.Hls) error

	// Delete a Hls by id.
	Delete(context.Context, uint) error
}

type hlsDao struct {
	db *gorm.DB
}

func NewHlsDao() HlsDao {
	return hlsDao{db: DB}
}

// Create a Hls stream.
func (d hlsDao) Create(c context.Context, it *model.Hls) error {
	return DB.WithContext(c).Create(it).Error
}

// Delete a Hls stream by id.
func (d hlsDao) Delete(c context.Context, id uint) error {
	return DB.WithContext(c).Delete(&model.Hls{}, id).Error
}
