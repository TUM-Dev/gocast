package model

import (
	"gorm.io/gorm"
)

type Poll struct {
	gorm.Model

	StreamID uint   // used by gorm
	Stream   Stream `gorm:"foreignKey:stream_id;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Question string `gorm:"not null" json:"question"`

	PollOptions []PollOption `gorm:"many2many:chat_poll_options" json:"-"`
}

type PollOption struct {
	gorm.Model

	Answer string `gorm:"not null" json:"answer"`
	Votes  []User `gorm:"many2many:poll_option_user_votes" json:"-"`
}
