package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=file.go -destination ../mock_dao/changelog.go

type ChangelogDao interface {
	NewChangelog(cl *model.Changelog) error
}

type changelogDao struct {
	db *gorm.DB
}

func NewChangelogDao() ChangelogDao {
	return &changelogDao{db: DB}
}

func (dao *changelogDao) NewChangelog(cl *model.Changelog) error {
	return dao.db.Create(cl).Error
}
