package dao

import (
	"gorm.io/gorm"
)

var (
	// DB reference to database
	DB *gorm.DB

	Migrator = newMigrator()
)
