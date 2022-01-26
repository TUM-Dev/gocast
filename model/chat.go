package model

import (
	"database/sql"
	"errors"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

var (
	ErrReplyToReply       = errors.New("reply to reply not allowed")
	ErrReplyToWrongStream = errors.New("reply to message from different stream not allowed")
	ErrReplyToNoMsg       = errors.New("reply to message not found")

	ErrMessageTooLong = errors.New("message too long")
	ErrMessageNoText  = errors.New("message has no text")
	ErrCooledDown     = errors.New("user is cooled down")
)

const (
	maxMessageLength = 200
	coolDown         = time.Minute * 2
	coolDownMessages = 5 // 5 messages -> 5 messages per 2 minutes max
)

type Chat struct {
	gorm.Model

	UserID   string `gorm:"not null" json:"-"`
	UserName string `gorm:"not null" json:"name"`
	Message  string `gorm:"not null" json:"message"`
	StreamID uint   `gorm:"not null" json:"-"`
	Admin    bool   `gorm:"not null;default:false" json:"admin"`
	Color    string `gorm:"not null;default:'#368bd6'" json:"color"`

	Replies []Chat        `gorm:"foreignkey:ReplyTo" json:"replies"`
	ReplyTo sql.NullInt64 `json:"replyTo"`
}

// getColors returns all colors chat names are mapped to
func (c Chat) getColors() []string {
	return []string{"#368bd6", "#ac3ba8", "#0dbd8b", "#e64f7a", "#ff812d", "#2dc2c5", "#5c56f5", "#74d12c"}
}

// BeforeCreate is a GORM hook that is called before a new chat is created.
// Messages won't be saved if any of these apply:
// - message is empty (after trimming)
// - message is too long (>maxMessageLength)
// - user is cooled down (user sent > coolDownMessages messages within coolDown)
// - message is a reply, and:
//   - reply is to a reply (not allowed)
//   - reply is to a message from a different stream
//   - reply is to a message that doesn't exist
func (c *Chat) BeforeCreate(tx *gorm.DB) (err error) {
	c.Message = strings.TrimSpace(c.Message)
	if len(c.Message) > maxMessageLength {
		return ErrMessageTooLong
	}
	if len(c.Message) == 0 {
		return ErrMessageNoText
	}
	var recentMessages int64
	err = tx.Model(&Chat{}).
		Where("created_at > ? AND user_id = ?", time.Now().Add(-coolDown), c.UserID).
		Count(&recentMessages).Error
	if err != nil {
		return err
	}
	if recentMessages >= coolDownMessages {
		return ErrCooledDown
	}

	// set chat color:
	colors := c.getColors()
	userIdInt, err := strconv.Atoi(c.UserID)
	if err != nil {
		c.Color = colors[0]
	} else {
		c.Color = colors[userIdInt%len(colors)]
	}

	// not a reply, no need for more checks
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
