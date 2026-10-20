package model

import "time"

// IntegrationGrant gives one registered application access to fixed metadata for
// one course. Revocation is permanent; reapproval creates a new grant ID.
type IntegrationGrant struct {
	ID            uint `gorm:"primaryKey"`
	IntegrationID uint `gorm:"not null;index"`
	CourseID      uint `gorm:"not null;index"`
	CreatedAt     time.Time
	RevokedAt     *time.Time
	Integration   Integration `gorm:"foreignKey:IntegrationID"`
}

// IntegrationAuthorizationCode is the one-use credential returned after consent.
// Only hashes of browser-provided values are retained.
type IntegrationAuthorizationCode struct {
	CodeHash      []byte `gorm:"type:binary(32);primaryKey"`
	StateHash     []byte `gorm:"type:binary(32);not null"`
	IntegrationID uint   `gorm:"not null;index"`
	GrantID       uint   `gorm:"not null;index"`
	ExpiresAt     time.Time
}

// IntegrationRecording is a lecture an integration created for one upload of its own,
// named by the integration's reference. Only the current upload ticket's hash is kept,
// so issuing a new ticket invalidates the previous one.
type IntegrationRecording struct {
	ID              uint   `gorm:"primaryKey"`
	IntegrationID   uint   `gorm:"not null;uniqueIndex:idx_integration_recording_ref"`
	ExternalRef     string `gorm:"type:varchar(64);not null;uniqueIndex:idx_integration_recording_ref"`
	GrantID         uint   `gorm:"not null"`
	StreamID        uint   `gorm:"not null;index"`
	TicketHash      []byte `gorm:"type:binary(32);uniqueIndex"`
	TicketExpiresAt time.Time
	// UploadStarted is set when the ticket is used: one lecture takes one upload.
	UploadStarted bool `gorm:"not null;default:false"`
	// Closed is set when the integration deletes the lecture; no ticket works after it.
	Closed bool `gorm:"not null;default:false"`
}
