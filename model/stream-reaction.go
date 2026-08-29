package model

import "gorm.io/gorm"

// StreamReaction represents Reactions of users to a Stream.
type StreamReaction struct {
	gorm.Model

	Reaction string `gorm:"not null" json:"reaction"`
	StreamID uint   `gorm:"not null;uniqueIndex:idx_stream_reaction_stream_user" json:"streamID"`
	UserID   uint   `gorm:"not null;uniqueIndex:idx_stream_reaction_stream_user" json:"userID"`
	// Name string `gorm:"column:name;type:text;not null;default:'unnamed'"`
}

// TableName returns the name of the table for the StreamReaction model in the database.
func (*StreamReaction) TableName() string {
	return "stream_reaction"
}
