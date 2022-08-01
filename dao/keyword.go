package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=keyword.go -destination ../mock_dao/keyword.go

type KeywordDao interface {
	NewKeywords(ctx context.Context, keyword []model.Keyword) error
}

type keywordDao struct {
	db *gorm.DB
}

func NewKeywordDao() KeywordDao {
	return keywordDao{db: DB}
}

func (d keywordDao) NewKeywords(ctx context.Context, keyword []model.Keyword) error {
	return DB.WithContext(ctx).Create(keyword).Error
}
