package model

import (
	"gorm.io/gorm"
)

type Course struct {
	gorm.Model

	UserID              uint
	Name                string
	Slug                string //eg. eidi
	TUMOnlineIdentifier string //not in use rn
	Streams             []Stream
}
