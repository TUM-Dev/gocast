package dao

import (
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=audit.go -destination ../mock_dao/audit.go

type AuditDao interface {
	// Create a new audit for the database
	Create(*model.Audit) error
	// Find audits
	Find(limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error)
	// Count the audits of the given types, so a client can page without probing.
	Count(types ...model.AuditType) (int64, error)
	// FindByCourse is Find for the entries about one course, its lectures included.
	FindByCourse(courseID uint, limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error)
	CountByCourse(courseID uint, types ...model.AuditType) (int64, error)
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

func (a auditDao) Count(types ...model.AuditType) (n int64, err error) {
	return n, a.db.Model(&model.Audit{}).Where("type in ?", types).Count(&n).Error
}

func (a auditDao) FindByCourse(courseID uint, limit int, offset int, types ...model.AuditType) (audits []model.Audit, err error) {
	return audits, a.db.
		Preload("User").
		Model(&model.Audit{}).
		Where("course_id = ? AND type in ?", courseID, types).
		// id breaks ties: entries written in the same second would otherwise swap
		// between pages.
		Order("created_at desc, id desc").
		Limit(limit).
		Offset(offset).
		Find(&audits).Error
}

func (a auditDao) CountByCourse(courseID uint, types ...model.AuditType) (n int64, err error) {
	return n, a.db.Model(&model.Audit{}).Where("course_id = ? AND type in ?", courseID, types).Count(&n).Error
}

func (a auditDao) Create(audit *model.Audit) error {
	return a.db.Create(audit).Error
}

func NewAuditDao() AuditDao {
	return auditDao{db: DB}
}
