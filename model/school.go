package model

import (
	"gorm.io/gorm"
)

// School represents an entity (e.g., A faculity, school, etc.) which needs additional priviledges and resource management capabilities.
type School struct {
	gorm.Model

	Name                   string   `gorm:"column:name;type:text;not null"` // e.g., Computation, Information and Technology
	University             string   `gorm:"column:university;type:text;not null;default:'unknown'"`
	SharedResourcesAllowed bool     `gorm:"column:shared_resources_allowed;type:boolean;not null;default:false"`
	Privileges             string   `gorm:"column:privileges;type:text;not null;default:''"`
	Admins                 []User   `gorm:"many2many:school_admins"`
	Courses                []Course `gorm:"foreignkey:SchoolID"`
	Workers                []Worker `gorm:"foreignkey:SchoolID"`
	Runners                []Runner `gorm:"foreignkey:SchoolID"`
	OrgId                  string   `gorm:"column:org_id;type:text;not null;default:''"`   // e.g., 51897
	OrgType                string   `gorm:"column:org_type;type:text;not null;default:''"` // e.g., TUM School
	OrgSlug                string   `gorm:"column:org_slug;type:text;not null;default:''"` // e.g., TUS1000
}

// Resources              []Resource `gorm:"many2many:resources"` // TODO: Contains workers etc.
// Departments []Department

/*type Department struct {
	ID       int
	Name     string
	TumOnlineId            string   `gorm:"column:tum_online_id;type:text;not null;default:''"`
	SchoolID int
	ChairID  int
}

type Chair struct {
	ID         int
	Name       string
	TumOnlineId            string   `gorm:"column:tum_online_id;type:text;not null;default:''"`
	Department Department
	Courses    []Course
}*/

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
