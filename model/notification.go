package model

import (
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
)

// NotificationTarget is a User group the Notification is displayed to
type NotificationTarget int

const (
	TargetAll      = iota + 1 // TargetAll Is any user, regardless if logged in or not
	TargetUser                // TargetUser Are all users that are logged in
	TargetStudent             // TargetStudent Are all users that are logged in and are students
	TargetLecturer            // TargetLecturer Are all users that are logged in and are lecturers
	TargetAdmin               // TargetAdmin Are all users that are logged in and are admins

)

// Notification is a message (e.g. a feature alert) that is displayed to users
type Notification struct {
	Model

	Title  *string            `json:"title,omitempty"`
	Body   string             `json:"-" gorm:"not null"`
	Target NotificationTarget `json:"-" gorm:"not null; default:1"`

	// SanitizedBody is the body of the notification, converted from markdown to HTML
	SanitizedBody string `json:"body" gorm:"-"`
}

// AfterFind populates the SanitizedBody after getting the Notification from the database
func (n *Notification) AfterFind(_ *gorm.DB) error {
	unsafe := blackfriday.Run([]byte(n.Body))
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	n.SanitizedBody = string(html)
	return nil
}

func (n *Notification) GetBodyForGoTemplate() template.HTML {
	return template.HTML(n.SanitizedBody)
}
