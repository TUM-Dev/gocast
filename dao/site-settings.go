package dao

import (
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

//go:generate go tool mockgen -source=site-settings.go -destination ../mock_dao/site-settings.go

type SiteSettingsDao interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	GetBool(key string) (bool, error)
	SetBool(key string, value bool) error
}

type siteSettingsDao struct {
	db *gorm.DB
}

func NewSiteSettingsDao() SiteSettingsDao {
	return siteSettingsDao{db: DB}
}

// Get retrieves a setting value by key
func (d siteSettingsDao) Get(key string) (string, error) {
	var setting model.SiteSetting
	err := d.db.Where("k = ?", key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.V, nil
}

// Set creates or updates a setting value
func (d siteSettingsDao) Set(key string, value string) error {
	var setting model.SiteSetting
	err := d.db.Where("k = ?", key).First(&setting).Error
	if err == gorm.ErrRecordNotFound {
		// Create new setting
		setting = model.SiteSetting{K: key, V: value}
		return d.db.Create(&setting).Error
	}
	if err != nil {
		return err
	}
	// Update existing setting
	setting.V = value
	return d.db.Save(&setting).Error
}

// GetBool retrieves a boolean setting value by key
func (d siteSettingsDao) GetBool(key string) (bool, error) {
	val, err := d.Get(key)
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

// SetBool sets a boolean setting value
func (d siteSettingsDao) SetBool(key string, value bool) error {
	strValue := "false"
	if value {
		strValue = "true"
	}
	return d.Set(key, strValue)
}
