package model

import (
	"database/sql"
	"errors"
	"gorm.io/gorm"
)

var ErrReplyToReply = errors.New("reply to reply not allowed")
var ErrReplyToWrongStream = errors.New("reply to message from different stream not allowed")
var ErrReplyToNoMsg = errors.New("reply to message not found")

type Chat struct {
	gorm.Model

	UserID   string `gorm:"not null"`
	UserName string `gorm:"not null"`
	Message  string `gorm:"not null"`
	StreamID uint   `gorm:"not null"`
	Admin    bool   `gorm:"not null;default:false"`

	Replies []Chat `gorm:"foreignkey:ReplyTo"`
	ReplyTo sql.NullInt64
}

func (c *Chat) BeforeCreate(tx *gorm.DB) (err error) {
	if !c.ReplyTo.Valid {
		return nil
	}
	var replyTo Chat
	if err = tx.First(&replyTo, c.ReplyTo).Error; err != nil {
		return ErrReplyToNoMsg // can't reply to non-existent message
	}
	if replyTo.StreamID != c.StreamID {
		return ErrReplyToWrongStream // can't reply to message from different stream
	}
	if replyTo.ReplyTo.Valid {
		return ErrReplyToReply // can't reply to reply
	}
	return nil
}
