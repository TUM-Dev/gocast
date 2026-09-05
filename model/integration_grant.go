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
