package dao

import (
	"context"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=integration.go -destination ../mock_dao/integration.go

type IntegrationDao interface {
	CreateIntegration(*model.Integration) error
	GetIntegrationByAPIKeyHash(context.Context, []byte) (model.Integration, error)
	GetIntegrations() ([]model.Integration, error)
	SetIntegrationAPIKey(uint, []byte) error
}

type integrationDao struct {
	db *gorm.DB
}

func NewIntegrationDao() IntegrationDao { return &integrationDao{db: DB} }

func (d integrationDao) CreateIntegration(integration *model.Integration) error {
	return d.db.Create(integration).Error
}

func (d integrationDao) GetIntegrationByAPIKeyHash(ctx context.Context, hash []byte) (model.Integration, error) {
	var integration model.Integration
	err := d.db.WithContext(ctx).Where("api_key_hash = ?", hash).First(&integration).Error
	return integration, err
}

func (d integrationDao) GetIntegrations() ([]model.Integration, error) {
	var integrations []model.Integration
	err := d.db.Order("name, id").Find(&integrations).Error
	return integrations, err
}

func (d integrationDao) SetIntegrationAPIKey(id uint, hash []byte) error {
	result := d.db.Model(&model.Integration{}).Where("id = ?", id).Update("api_key_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
