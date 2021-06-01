package model

import "gorm.io/gorm"

//ShortLink friendly name for a link to courses highlight page
type ShortLink struct {
	gorm.Model

	Link     string `gorm:"unique"`
	CourseId uint
}
