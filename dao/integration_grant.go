package dao

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=integration_grant.go -destination ../mock_dao/integration_grant.go

type IntegrationGrantDao interface {
	GetIntegrationByID(context.Context, uint) (model.Integration, error)
	GetAuthorizableCourses(context.Context, uint) ([]model.Course, error)
	GetCourseForAuthorization(context.Context, uint) (model.Course, error)
	ApproveIntegrationCourse(context.Context, uint, uint, []byte, []byte, time.Time) (uint, error)
	RedeemIntegrationAuthorizationCode(context.Context, uint, []byte, []byte, time.Time) (uint, uint, error)
	GetCourseIntegrationGrants(context.Context, uint) ([]model.IntegrationGrant, error)
	RevokeIntegrationGrant(context.Context, uint, uint) error
}

type integrationGrantDao struct {
	db *gorm.DB
}

func NewIntegrationGrantDao() IntegrationGrantDao { return &integrationGrantDao{db: DB} }

func (d integrationGrantDao) RedeemIntegrationAuthorizationCode(ctx context.Context, integrationID uint, codeHash, stateHash []byte, now time.Time) (grantID, courseID uint, err error) {
	err = d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var code model.IntegrationAuthorizationCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ?", codeHash).First(&code).Error; err != nil {
			return err
		}
		if code.IntegrationID != integrationID || subtle.ConstantTimeCompare(code.StateHash, stateHash) != 1 || !code.ExpiresAt.After(now) {
			return gorm.ErrRecordNotFound
		}
		var grant model.IntegrationGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND integration_id = ? AND revoked_at IS NULL", code.GrantID, integrationID).First(&grant).Error; err != nil {
			return err
		}
		result := tx.Where("code_hash = ?", codeHash).Delete(&model.IntegrationAuthorizationCode{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		grantID, courseID = grant.ID, grant.CourseID
		return nil
	})
	return grantID, courseID, err
}

func (d integrationGrantDao) GetIntegrationByID(ctx context.Context, id uint) (model.Integration, error) {
	var integration model.Integration
	err := d.db.WithContext(ctx).First(&integration, id).Error
	return integration, err
}

// Unlike the cached administered-course list, approval needs a current owner or explicit course administrator.
func (d integrationGrantDao) GetAuthorizableCourses(ctx context.Context, userID uint) ([]model.Course, error) {
	var courses []model.Course
	err := d.db.WithContext(ctx).Distinct("courses.*").
		Joins("LEFT JOIN course_admins ON course_admins.course_id = courses.id").
		Where("courses.user_id = ? OR course_admins.user_id = ?", userID, userID).
		Order("courses.year DESC, courses.teaching_term DESC, courses.name, courses.id").Find(&courses).Error
	return courses, err
}

func (d integrationGrantDao) GetCourseForAuthorization(ctx context.Context, courseID uint) (model.Course, error) {
	var course model.Course
	err := d.db.WithContext(ctx).Preload("Admins").First(&course, courseID).Error
	return course, err
}

func (d integrationGrantDao) ApproveIntegrationCourse(ctx context.Context, integrationID, courseID uint, codeHash, stateHash []byte, expiresAt time.Time) (grantID uint, err error) {
	err = d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var integration model.Integration
		// Serialize approvals so concurrent requests reuse one active grant.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&integration, integrationID).Error; err != nil {
			return err
		}
		var grant model.IntegrationGrant
		err := tx.Where("integration_id = ? AND course_id = ? AND revoked_at IS NULL", integrationID, courseID).
			Order("id").First(&grant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			grant = model.IntegrationGrant{IntegrationID: integrationID, CourseID: courseID}
			if err := tx.Create(&grant).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		code := model.IntegrationAuthorizationCode{
			CodeHash: codeHash, StateHash: stateHash, IntegrationID: integrationID,
			GrantID: grant.ID, ExpiresAt: expiresAt,
		}
		if err := tx.Create(&code).Error; err != nil {
			return err
		}
		grantID = grant.ID
		return nil
	})
	return grantID, err
}

func (d integrationGrantDao) GetCourseIntegrationGrants(ctx context.Context, courseID uint) ([]model.IntegrationGrant, error) {
	var grants []model.IntegrationGrant
	err := d.db.WithContext(ctx).Preload("Integration").
		Where("course_id = ? AND revoked_at IS NULL", courseID).Order("created_at, id").Find(&grants).Error
	return grants, err
}

func (d integrationGrantDao) RevokeIntegrationGrant(ctx context.Context, grantID, courseID uint) error {
	result := d.db.WithContext(ctx).Model(&model.IntegrationGrant{}).
		Where("id = ? AND course_id = ? AND revoked_at IS NULL", grantID, courseID).
		Update("revoked_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
