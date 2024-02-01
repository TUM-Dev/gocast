package model

import (
	"database/sql"

	"gorm.io/gorm"
)

const TokenScopeAdmin = "admin"

// Token can be used to authenticate instead of a user account
type Token struct {
	gorm.Model
	UserID  uint         // used by gorm
	User    User         `gorm:"foreignKey:user_id;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // creator of the token
	Token   string       `json:"token" gorm:"not null"`                                                     // secret token
	Expires sql.NullTime `json:"expires"`                                                                   // expiration date (null if none)
	Scope   string       `json:"scope" gorm:"not null"`                                                     // scope of the token, currently only admin
	LastUse sql.NullTime `json:"last_use"`                                                                  // last time the token was used
}
