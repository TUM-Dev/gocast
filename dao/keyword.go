package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=keyword.go -destination ../mock_dao/keyword.go

type KeywordDao interface {
	NewKeywords(keyword []model.Keyword) error
}

type keywordDao struct {
	db *gorm.DB
}

func NewKeywordDao() KeywordDao {
	return keywordDao{db: DB}
}

func (d keywordDao) NewKeywords(keyword []model.Keyword) error {
	return DB.Create(keyword).Error
}
