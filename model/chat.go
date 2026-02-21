package model

import (
	"database/sql"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"gorm.io/gorm"
	"mvdan.cc/xurls/v2"
)

var (
	ErrReplyToReply       = errors.New("reply to reply not allowed")
	ErrReplyToWrongStream = errors.New("reply to message from different stream not allowed")
	ErrReplyToNoMsg       = errors.New("reply to message not found")

	ErrMessageTooLong = errors.New("message too long")
	ErrMessageNoText  = errors.New("message has no text")
	ErrCooledDown     = errors.New("user is cooled down")
)

var (
	// chatHTMLPolicy defines the policy for sanitizing chat messages
	chatHTMLPolicy = bluemonday.StrictPolicy().AllowAttrs("href").
			OnElements("a").
			AllowURLSchemes("http", "https").
			AllowRelativeURLs(false).
			RequireNoFollowOnLinks(true).
			AddTargetBlankToFullyQualifiedLinks(true).
			RequireParseableURLs(true)
	chatURLPolicy = xurls.Strict() // require protocol for URLs
)

const (
	maxMessageLength = 1000
	coolDown         = time.Minute * 2
	coolDownMessages = 5 // 5 messages -> 5 messages per 2 minutes max
)

type Chat struct {
	gorm.Model

	UserID           string `gorm:"not null" json:"userId"`
	UserName         string `gorm:"not null" json:"name"`
	Message          string `gorm:"type:text;not null;index:,class:FULLTEXT" json:"-"`
	SanitizedMessage string `gorm:"-" json:"message"` // don't store the sanitized message in the database
	StreamID         uint   `gorm:"not null" json:"-"`
	Admin            bool   `gorm:"not null;default:false" json:"admin"`
	Color            string `gorm:"not null;default:'#368bd6'" json:"color"`

	Visible   sql.NullBool `gorm:"not null;default:true" json:"-"`
	IsVisible bool         `gorm:"-" json:"visible"` // IsVisible is .Bool value of Visible for simplicity

	Reactions []ChatReaction `gorm:"foreignKey:chat_id;" json:"reactions"`

	AddressedToUsers []User `gorm:"many2many:chat_user_addressedto" json:"-"`
	AddressedToIds   []uint `gorm:"-" json:"addressedTo"`

	Replies []Chat        `gorm:"foreignkey:ReplyTo" json:"replies"`
	ReplyTo sql.NullInt64 `json:"replyTo"`

	Resolved bool `gorm:"not null;default:false" json:"resolved"`
}

// getColors returns all colors chat names are mapped to
func getColors() []string {
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
	if !c.Admin {
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
	}

	// set chat color:
	colors := getColors()
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

// AfterFind is a GORM hook that sanitizes the message after it's loaded from the database.
func (c *Chat) AfterFind(_ *gorm.DB) (err error) {
	c.SanitiseMessage()
	c.IsVisible = c.Visible.Bool
	return nil
}

// getUrlHtml returns the html for urls, the <a> tag includes target="_blank" and rel="nofollow noopener"
func getUrlHtml(url string) string {
	h := blackfriday.Run([]byte(url))
	return strings.TrimSuffix(string(chatHTMLPolicy.SanitizeBytes(h)), "\n")
}

// SanitiseMessage sets chat.SanitizedMessage to the sanitized html version of chat.Message, including <a> tags for links
func (c *Chat) SanitiseMessage() {
	msg := html.EscapeString(c.Message)
	urls := chatURLPolicy.FindAllStringIndex(msg, -1)
	newMsg := ""
	for _, urlIndex := range urls {
		newMsg += msg[:urlIndex[0]]
		newMsg += getUrlHtml(msg[urlIndex[0]:urlIndex[1]])
	}
	if len(urls) > 0 {
		newMsg += msg[urls[len(urls)-1][1]:]
	} else {
		newMsg = msg
	}
	c.SanitizedMessage = newMsg
}
