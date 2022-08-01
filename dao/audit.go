package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=audit.go -destination ../mock_dao/audit.go

type AuditDao interface {
	// Create a new audit for the database
	Create(ctx context.Context, audit *model.Audit) error
	// Find audits
	Find(ctx context.Context, limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error)
}

type auditDao struct {
	db *gorm.DB
}

func (a auditDao) Find(ctx context.Context, limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error) {
	return audits, a.db.WithContext(ctx).
		Preload("User").
		Model(&model.Audit{}).
		Where("type in ?", types).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&audits).Error
}

func (a auditDao) Create(ctx context.Context, audit *model.Audit) error {
	return a.db.WithContext(ctx).Create(audit).Error
}

func NewAuditDao() AuditDao {
	return auditDao{db: DB}
}
