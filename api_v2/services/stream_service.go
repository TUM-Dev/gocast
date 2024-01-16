// Package services provides functions for fetching data from the database.
package services

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

// GetStreamByID retrieves a stream by its id.
func GetStreamByID(db *gorm.DB, streamID uint) (*model.Stream, error) {
	s := &model.Stream{}
	err := db.Where("streams.id = ?", streamID).First(s).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

	return s, nil
}

// GetStreamsByCourseID retrieves all streams of a course by its id.
func GetStreamsByCourseID(db *gorm.DB, courseID uint) ([]*model.Stream, error) {
	var streams []*model.Stream
	if err := db.Where("streams.course_id = ?", courseID).Find(&streams).Error; err != nil {
		return nil, err
	}

	return streams, nil
}

func GetEnrolledOrPublicLiveStreams(db *gorm.DB, uID *uint) ([]*model.Stream, error) {
	var streams []*model.Stream
	if *uID == 0 {
		err := db.Table("streams").
			Joins("join courses on streams.course_id = courses.id").
			Joins("left join course_users on courses.id = course_users.course_id").
			Where("(course_users.user_id = ? OR courses.visibility = \"public\") AND streams.live_now = 1", *uID).
			Find(&streams).Error
		if err != nil {
			return nil, err
		}
	} else {
		err := db.Table("streams").
			Select("DISTINCT streams.*").
			Joins("join courses on streams.course_id = courses.id").
			Joins("left join course_users on courses.id = course_users.course_id").
			Where("(course_users.user_id = ? OR courses.visibility = \"public\" OR courses.visibility = \"loggedin\") AND streams.live_now = 1", *uID).
			Find(&streams).Error
		if err != nil {
			return nil, err
		}
	}

	return streams, nil
}

// GetProgress retrieves the progress of a stream for a user.
func GetProgress(db *gorm.DB, streamID uint, userID uint) (*model.StreamProgress, error) {
	p := &model.StreamProgress{}

	err := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("progress not found"))
	}

	return p, nil
}

func SetProgress(db *gorm.DB, streamID uint, userID uint, progress float64) (*model.StreamProgress, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	if progress < 0 || progress > 1 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("progress must be between 0 and 1"))
	}

	p := &model.StreamProgress{}

	result := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		p.StreamID = streamID
		p.UserID = userID
		p.Progress = progress
		p.Watched = progress == 1
	case result.Error != nil:
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	default:
		p.Progress = progress
		p.Watched = progress == 1
	}

	if err := db.Save(p).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return p, nil
}

func MarkAsWatched(db *gorm.DB, streamID uint, userID uint) (*model.StreamProgress, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	p := &model.StreamProgress{}

	result := db.Where("stream_id = ? AND user_id = ?", streamID, userID).First(p)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		p.StreamID = streamID
		p.UserID = userID
		p.Progress = 1
		p.Watched = true
	case result.Error != nil:
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	default:
		p.Progress = 1
		p.Watched = true
	}

	if err := db.Save(p).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return p, nil
}

func GetChatMessages(db *gorm.DB, streamID uint) ([]*model.Chat, error) {
	var chats []*model.Chat

	// chats which are replies should not be listed in the chat list itself only in the replies of the chat they are replying to
	// also preload reactions and replies of chats
	if err := db.Preload("Reactions").Preload("Replies").Where("stream_id = ? AND reply_to IS NULL", streamID).Find(&chats).Error; err != nil {
		return nil, err
	}

	return chats, nil
}

func PostChatMessage(db *gorm.DB, streamID uint, userID uint, message string) (*model.Chat, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	user := &model.User{}
	err = db.Where("id = ?", userID).First(user).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	coolDown := 10 * time.Second
	coolDownMessages := 1

	var recentMessages int64
	err = db.Model(&model.Chat{}).
		Where("created_at > ? AND user_id = ?", time.Now().Add(-coolDown), userID).
		Count(&recentMessages).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if recentMessages >= int64(coolDownMessages) {
		return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("user is posting too fast"))
	}

	var recentMessagesWithSameContent int64
	err = db.Model(&model.Chat{}).
		Where("created_at > ? AND user_id = ? AND message = ?", time.Now().Add(-60*time.Second), userID, message).
		Count(&recentMessagesWithSameContent).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if recentMessagesWithSameContent >= int64(coolDownMessages) {
		return nil, e.WithStatus(http.StatusTooManyRequests, errors.New("user is posting the same message too fast"))
	}

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// role is 1 or 2 means lecturer or admin
	isAdmin := user.Role == 1 || user.Role == 2

	// TODO: address users using @ figure out reply to field and addressed to ids

	c := &model.Chat{
		UserID:   strconv.Itoa(int(userID)),
		UserName: user.Name,
		Message:  message,
		StreamID: streamID,
		Admin:    isAdmin,
	}

	if err := db.Save(c).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return c, nil
}

func PostChatReaction(db *gorm.DB, streamID uint, userID uint, chatID uint, reaction string) (*model.ChatReaction, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	// TODO check reaction is valid (1 singular emoji)
	// this is not so easy since skin tones, or flags are composed of a variety of emojis/ unicode characters

	existingChat := &model.Chat{}
	err = db.Where("id = ?", chatID).First(existingChat).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("chat not found"))
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	user := &model.User{}

	err = db.Where("id = ?", userID).First(user).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	reactionModel := &model.ChatReaction{
		ChatID:   chatID,
		UserID:   userID,
		Username: user.Name,
		Emoji:    reaction,
	}

	result := db.Where("chat_id = ? AND user_id = ?", chatID, userID).First(reactionModel)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		if err := db.Save(reactionModel).Error; err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		return reactionModel, nil
	case result.Error != nil:
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	default:
		if err := db.Model(reactionModel).Update("emoji", reaction).Error; err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
	}
	return reactionModel, nil
}

func DeleteChatReaction(db *gorm.DB, streamID uint, userID uint, chatID uint) (*model.ChatReaction, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	reactionModel := &model.ChatReaction{}

	result := db.Where("chat_id = ? AND user_id = ?", chatID, userID).First(reactionModel)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		return nil, e.WithStatus(http.StatusNotFound, errors.New("reaction not found"))
	case result.Error != nil:
		return nil, e.WithStatus(http.StatusInternalServerError, result.Error)
	default:
		if err := db.Delete(reactionModel).Error; err != nil {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}
		return reactionModel, nil
	}
}

func PostChatReply(db *gorm.DB, streamID uint, userID uint, chatID uint, message string) (*model.Chat, error) {
	_, err := GetStreamByID(db, streamID)
	if err != nil {
		return nil, err
	}

	// create chat and attach it to the chat with chatID
	existingThread := &model.Chat{}
	err = db.Where("id = ?", chatID).First(existingThread).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	user := &model.User{}
	err = db.Where("id = ?", userID).First(user).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// role is 1 or 2 means lecturer or admin
	isAdmin := user.Role == 1 || user.Role == 2

	c := &model.Chat{
		UserID:   strconv.Itoa(int(userID)),
		UserName: user.Name,
		Message:  message,
		StreamID: streamID,
		Admin:    isAdmin,
		ReplyTo:  sql.NullInt64{Int64: int64(chatID), Valid: true},
	}

	if err := db.Save(c).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	existingThread.Replies = append(existingThread.Replies, *c)

	if err := db.Save(existingThread).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return c, nil
}

func MarkChatMessageAsResolved(db *gorm.DB, userID uint, chatID uint) (*model.Chat, error) {
	// find chat check if user owns it or is admin
	chat := &model.Chat{}

	err := db.Where("id = ?", chatID).First(chat).Error
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	user := &model.User{}
	err = db.Where("id = ?", userID).First(user).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// role is 1 or 2 means lecturer or admin
	isAdmin := user.Role == 1 || user.Role == 2

	if chat.UserID != strconv.Itoa(int(userID)) && !isAdmin {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("user is not allowed to resolve this chat message"))
	}

	if chat.Resolved {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("chat message is already resolved"))
	}

	chat.Resolved = true

	if err := db.Save(chat).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return chat, nil
}

func MarkChatMessageAsUnresolved(db *gorm.DB, userID uint, chatID uint) (*model.Chat, error) {
	// find chat check if user owns it or is admin

	chat := &model.Chat{}

	err := db.Where("id = ?", chatID).First(chat).Error
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	user := &model.User{}
	err = db.Where("id = ?", userID).First(user).Error

	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// role is 1 or 2 means lecturer or admin
	isAdmin := user.Role == 1 || user.Role == 2

	if chat.UserID != strconv.Itoa(int(userID)) && !isAdmin {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("user is not allowed to resolve this chat message"))
	}

	if !chat.Resolved {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("chat message is already unresolved"))
	}

	chat.Resolved = false

	if err := db.Save(chat).Error; err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return chat, nil
}
