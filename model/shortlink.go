package model

import "gorm.io/gorm"

// ShortLink friendly name for a link to a course; it redirects to the course's current or next lecture
type ShortLink struct {
	gorm.Model

	Link     string `gorm:"type:varchar(256); unique; not null"`
	CourseId uint   `gorm:"not null"`
}
