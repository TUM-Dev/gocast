package model

import (
	"gorm.io/gorm"
)

// School represents an entity (e.g., A faculity, school, etc.) which needs additional priviledges and resource management capabilities.
type School struct {
	gorm.Model

	Name                   string   `gorm:"column:name;type:text;not null"`
	University             string   `gorm:"column:university;type:text;not null;default:'unknown'"`
	SharedResourcesAllowed bool     `gorm:"column:shared_resources_allowed;type:boolean;not null;default:false"`
	Privileges             string   `gorm:"column:privileges;type:text;not null;default:''"`
	Admins                 []User   `gorm:"many2many:school_admins"`
	Courses                []Course `gorm:"foreignkey:SchoolID"`
	TumOnlineId            string   `gorm:"column:tum_online_id;type:text;not null;default:''"` // Used to identify corresponding TUMOnline group (e.g., TU0001...)
	// Resources              []Resource `gorm:"many2many:resources"` // TODO: Contains workers etc.
}

// TableName returns the name of the table for the School model in the database.
func (*School) TableName() string {
	return "schools" // todo
}

// BeforeCreate todo
func (s *School) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (s *School) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
