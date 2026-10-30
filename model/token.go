package model

import (
	"database/sql"

	"gorm.io/gorm"
)

// TokenScopeLecturer is the only scope left: it lets rtmp-proxy swap the token for the
// stream key of its holder's next stream. An "admin" scope that authenticated API
// scripts was removed; migration 202610300 deletes the rows that still carried it.
const TokenScopeLecturer = "lecturer"

// Token is what a lecturer's streaming client presents to rtmp-proxy instead of the
// stream key
type Token struct {
	gorm.Model
	UserID  uint         // used by gorm
	User    User         `gorm:"foreignKey:user_id;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // creator of the token
	Token   string       `json:"token" gorm:"not null"`                                                     // secret token
	Expires sql.NullTime `json:"expires"`                                                                   // expiration date (null if none)
	Scope   string       `json:"scope" gorm:"not null"`                                                     // scope of the token, currently only lecturer
	LastUse sql.NullTime `json:"last_use"`                                                                  // last time the token was used
}
