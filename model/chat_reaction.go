package model

type ChatReaction struct {
	ChatID   uint   `gorm:"primaryKey; not null" json:"chatID"`
	UserID   uint   `gorm:"primaryKey; not null" json:"userID"`
	Username string `gorm:"not null" json:"username"`
	Emoji    string `gorm:"primaryKey; not null" json:"emoji"`

	UserOAuthID string `gorm:"not null; type:varchar(50)"` // User's OAuth ID
}
