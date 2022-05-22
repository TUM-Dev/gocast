package model

import "gorm.io/gorm"

type Keyword struct {
	gorm.Model

	StreamID uint   `gorm:"not null"`
	Text     string `gorm:"text;not null;index:,class:FULLTEXT"`
	Language string `gorm:"not null"`
}
