package model

import "gorm.io/gorm"

// Runner represents a runner handling streams, converting videos,
// extracting silence from audios, creating thumbnails, etc.
type Runner struct {
	Hostname string `gorm:"column:hostname;primaryKey;unique;not null"`
	Port     uint32 `gorm:"column:port;not null"`
}

// TableName returns the name of the table for the Runner model in the database.
func (*Runner) TableName() string {
	return "runners" // todo
}

// BeforeCreate returns an error, if Runner r is invalid
func (r *Runner) BeforeCreate(tx *gorm.DB) (err error) {
	// this method currently is a noop
	return nil
}

// AfterFind runs after the Runner is fetched from the db
func (r *Runner) AfterFind(tx *gorm.DB) (err error) {
	// this method currently is a noop
	return nil
}
