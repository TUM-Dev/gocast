package dao

import (
	"context"
	"time"

	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=email.go -destination ../mock_dao/email.go

type EmailDao interface {
	// Get Email by ID
	Get(context.Context, uint) (model.Email, error)

	// Create an new Email for the database
	Create(context.Context, *model.Email) error

	// Delete an Email by id.
	Delete(context.Context, uint) error

	// Save an Email by id.
	Save(context.Context, *model.Email) error

	// GetDue Gets a number of emails that is due for sending.
	GetDue(context.Context, int) ([]model.Email, error)

	// GetFailed Gets all failed sending attempts.
	GetFailed(context.Context) ([]model.Email, error)
}

type emailDao struct {
	db *gorm.DB
}

func NewEmailDao() EmailDao {
	return emailDao{db: DB}
}

// Get an Email by id.
func (d emailDao) Get(c context.Context, id uint) (res model.Email, err error) {
	return res, DB.WithContext(c).First(&res, id).Error
}

// Create an Email.
func (d emailDao) Create(c context.Context, it *model.Email) error {
	return DB.WithContext(c).Create(it).Error
}

// Delete an Email by id.
func (d emailDao) Delete(c context.Context, id uint) error {
	return DB.WithContext(c).Delete(&model.Email{}, id).Error
}

// Save an Email.
func (d emailDao) Save(c context.Context, m *model.Email) error {
	return DB.WithContext(c).Save(m).Error
}

// GetDue Gets a number of emails that is due for sending.
func (d emailDao) GetDue(c context.Context, limit int) (res []model.Email, err error) {
	return res,
		DB.
			WithContext(c).
			Where("success = false AND retries <= 10 AND (last_try IS NULL OR last_try < ?)", time.Now().Add(-time.Minute)).
			Limit(limit).
			Find(&res).
			Error
}

// GetFailed Gets all failed sending attempts.
func (d emailDao) GetFailed(c context.Context) (res []model.Email, err error) {
	return res,
		DB.
			WithContext(c).
			Where("success = false").
			Find(&res).
			Error
}
