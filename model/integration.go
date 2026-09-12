package model

import "gorm.io/gorm"

// Integration is a registered external application with its own API key.
type Integration struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"type:varchar(100);not null" json:"name"`
	ReturnURL  string `gorm:"type:varchar(2048);not null" json:"returnUrl"`
	APIKeyHash []byte `gorm:"type:binary(32);uniqueIndex" json:"-"`
	KeyActive  bool   `gorm:"-" json:"hasKey"`
}

func (i *Integration) AfterFind(*gorm.DB) error {
	i.KeyActive = len(i.APIKeyHash) != 0
	return nil
}
