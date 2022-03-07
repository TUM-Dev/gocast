package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Poll struct {
	gorm.Model

	StreamID uint   // used by gorm
	Stream   Stream `gorm:"foreignKey:stream_id;not null;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Question string `gorm:"not null" json:"question"`
	Active   bool   `gorm:"not null;default:true" json:"active"`

	PollOptions []PollOption `gorm:"many2many:chat_poll_options" json:"pollOptions"`
}

type PollOption struct {
	gorm.Model

	Answer string `gorm:"not null" json:"answer"`
	Votes  []User `gorm:"many2many:poll_option_user_votes" json:"-"`
}

func (p Poll) GetPollOptionsJSON() []gin.H {
	var pollOptions []gin.H
	for _, option := range p.PollOptions {
		pollOptions = append(pollOptions, gin.H{
			"ID":     option.ID,
			"answer": option.Answer,
		})
	}
	return pollOptions
}
