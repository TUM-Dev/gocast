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

func GetAllAuditTypes() []AuditType {
	return []AuditType{
		AuditInfo,
		AuditWarning,
		AuditError,
		AuditCourseCreate,
		AuditCourseEdit,
		AuditCourseDelete,
		AuditStreamCreate,
		AuditStreamEdit,
		AuditStreamDelete,
		AuditCameraMoved,
	}
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
