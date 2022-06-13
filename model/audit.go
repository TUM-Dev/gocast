package model

import "gorm.io/gorm"

type AuditType uint

const (
	AuditInfo AuditType = iota + 1
	AuditWarning
	AuditError
	AuditCourseCreate
	AuditCourseEdit
	AuditCourseDelete
	AuditStreamCreate
	AuditStreamEdit
	AuditStreamDelete
	AuditCameraMoved
)

// String returns a string representation of the AuditType
func (t AuditType) String() string {
	return []string{
		"Info",
		"Warning",
		"Error",
		"Course Created",
		"Course Edited",
		"Course Deleted",
		"Stream Created",
		"Stream Edited",
		"Stream Deleted",
		"Camera Moved",
	}[t-1]
}

type Audit struct {
	gorm.Model

	User    *User // if nil -> system
	UserID  *uint
	Message string
	Type    AuditType
}
