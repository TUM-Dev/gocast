package model

import (
	"gorm.io/gorm"
)

// Organization represents an entity (e.g., A faculity, organization, etc.) which needs additional priviledges and resource management capabilities.
type Organization struct {
	gorm.Model

	Name            string         `gorm:"column:name;type:text;not null"` // e.g., Computation, Information and Technology
	Privileges      string         `gorm:"column:privileges;type:text;not null;default:''"`
	Admins          []User         `gorm:"many2many:organization_admins"`
	Courses         []Course       `gorm:"foreignkey:OrganizationID"`
	Workers         []Worker       `gorm:"foreignkey:OrganizationID"`
	Runners         []Runner       `gorm:"foreignkey:OrganizationID"`
	IngestServers   []IngestServer `gorm:"foreignkey:OrganizationID"`
	OrgId           string         `gorm:"column:org_id;type:text;not null;default:''"`                   // e.g., 51897
	OrgType         string         `gorm:"column:org_type;type:text;not null;default:'sub-organization'"` // e.g., TUM Organization
	OrgSlug         string         `gorm:"column:org_slug;type:text;not null;default:''"`                 // e.g., TUS1000
	ParentID        uint           `gorm:"column:parent_id;type:integer;not null;default:0"`
	IngestServerURL string         `gorm:"column:ingest_server_url;type:text;default:''"`
}

// Resources              []Resource `gorm:"many2many:resources"` // TODO: Contains workers etc.
// Departments []Department

/*type Department struct {
	ID       int
	Name     string
	TumOnlineId            string   `gorm:"column:tum_online_id;type:text;not null;default:''"`
	OrganizationID int
	ChairID  int
}

type Chair struct {
	ID         int
	Name       string
	TumOnlineId            string   `gorm:"column:tum_online_id;type:text;not null;default:''"`
	Department Department
	Courses    []Course
}*/

// TableName returns the name of the table for the Organization model in the database.
func (*Organization) TableName() string {
	return "organizations" // todo
}

// BeforeCreate todo
func (s *Organization) BeforeCreate(tx *gorm.DB) (err error) {
	return nil
}

// AfterFind todo
func (s *Organization) AfterFind(tx *gorm.DB) (err error) {
	return nil
}
