package dao

import (
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=audit.go -destination ../mock_dao/audit.go

type AuditDao interface {
	// Create a new audit for the database
	Create(*model.Audit) error
	// Find audits
	Find(limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error)
}

type auditDao struct {
	db *gorm.DB
}

func (a auditDao) Find(limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error) {
	return audits, a.db.
		Preload("User").
		Model(&model.Audit{}).
		Where("type in ?", types).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&audits).Error
}

func (a auditDao) Create(audit *model.Audit) error {
	return a.db.Create(audit).Error
}

func NewAuditDao() AuditDao {
	return auditDao{db: DB}
}
