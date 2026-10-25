package model

import (
	"github.com/gin-gonic/gin"
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
}

// Json converts the audit into a json object consumed by apis
func (a Audit) Json() gin.H {
	return gin.H{
		"type":      a.Type.String(),
		"createdAt": a.CreatedAt.Format("Jan 02, 2006: 15:04:05"),
		"id":        a.ID,
		"message":   a.Message,
		"userID":    a.UserID,
		"userName":  a.User.GetLoginString(),
	}
}
