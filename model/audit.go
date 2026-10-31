package model

import (
	"gorm.io/gorm"
)

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
	AuditImpersonation
	// Appended after AuditImpersonation: the type is persisted as its integer, so the
	// order of the constants above must never change.
	AuditUserCreate
	AuditUserEdit
	AuditUserDelete
	AuditTokenCreate
	AuditTokenDelete
	AuditIntegration
	AuditLectureHall
	AuditRunner
	AuditMaintenance
	AuditContent
)

var auditTypeNames = [...]string{
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
	"Impersonation",
	"User Created",
	"User Edited",
	"User Deleted",
	"Token Created",
	"Token Deleted",
	"Integration",
	"Lecture Hall",
	"Runner",
	"Maintenance",
	"Content",
}

// String returns a string representation of the AuditType. A value outside the known
// range (a row written by a newer version) renders as "Unknown" rather than panicking
// the whole audit page.
func (t AuditType) String() string {
	if t < 1 || int(t) > len(auditTypeNames) {
		return "Unknown"
	}
	return auditTypeNames[t-1]
}

func GetAllAuditTypes() []AuditType {
	types := make([]AuditType, 0, len(auditTypeNames))
	for i := range auditTypeNames {
		types = append(types, AuditType(i+1))
	}
	return types
}

// AuditTypeByName is the inverse of String, for filtering by the label clients see.
func AuditTypeByName(name string) (AuditType, bool) {
	for i, n := range auditTypeNames {
		if n == name {
			return AuditType(i + 1), true
		}
	}
	return 0, false
}

type Audit struct {
	gorm.Model

	User    *User // if nil -> system
	UserID  *uint
	Message string
	Type    AuditType

	// The course and lecture the entry is about, nil for server-wide entries. Rows
	// from before these columns existed are linked by dao/migrations where the
	// message allows it. Plain ids, no foreign keys: courses and streams are
	// soft-deleted and their history should outlive them.
	CourseID *uint `gorm:"index"`
	StreamID *uint
}
