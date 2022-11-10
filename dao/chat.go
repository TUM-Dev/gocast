package dao

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=chat.go -destination ../mock_dao/chat.go

type ChatDao interface {
	AddChatPollOptionVote(pollOptionId uint, userId uint) error
	AddChatPoll(poll *model.Poll) error
	AddMessage(chat *model.Chat) error

	GetChatUsers(streamid uint) ([]model.User, error)
	GetNumLikes(chatID uint) (int64, error)
	GetVisibleChats(userID uint, streamID uint) ([]model.Chat, error)
	GetAllChats(userID uint, streamID uint) ([]model.Chat, error)
	GetActivePoll(streamID uint) (model.Poll, error)
	GetPollUserVote(pollId uint, userId uint) (uint, error)
	GetPollOptionVoteCount(pollOptionId uint) (int64, error)

	ApproveChat(id uint) error
	DeleteChat(id uint) error
	ResolveChat(id uint) error
	ToggleLike(userID uint, chatID uint) error

	CloseActivePoll(streamID uint) error

	GetChatsByUser(userID uint) ([]model.Chat, error)
	GetChat(id uint, userID uint) (*model.Chat, error)
}

type chatDao struct {
	db *gorm.DB
}

func NewChatDao() ChatDao {
	return chatDao{db: DB}
}

func (d chatDao) AddChatPollOptionVote(pollOptionId uint, userId uint) error {
	return DB.Exec("INSERT INTO poll_option_user_votes (poll_option_id, user_id) VALUES (?, ?)", pollOptionId, userId).Error
}

func (d chatDao) AddChatPoll(poll *model.Poll) error {
	return DB.Save(poll).Error
}

func (d chatDao) AddMessage(chat *model.Chat) error {
	err := DB.Save(chat).Error
	if err != nil {
		return err
	}
	for _, userId := range chat.AddressedToIds {
		err := DB.Exec("INSERT INTO chat_user_addressedto (chat_id, user_id) VALUES (?, ?)", chat.ID, userId).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (d chatDao) GetChatUsers(streamid uint) ([]model.User, error) {
	var users []model.User
	query := DB.Model(&model.User{}).Select("distinct users.*").Joins("join chats c on c.user_id = users.id")
	query.Where("c.stream_id = ? AND c.user_name <> ?", streamid, "Anonymous")
	err := query.Scan(&users).Error
	if users == nil { // If no messages have been sent
		users = []model.User{}
	}
	return users, err
}

// GetNumLikes returns the number of likes for a message
func (d chatDao) GetNumLikes(chatID uint) (int64, error) {
	var numLikes int64
	err := DB.Table("chat_user_likes").Where("chat_id = ?", chatID).Count(&numLikes).Error
	return numLikes, err
}

// GetVisibleChats returns all visible chats for the stream with the given ID
// or sent by user with id 'userID'
// Number of likes are inserted and the user's like status is determined
func (d chatDao) GetVisibleChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers")
	query.Where("(visible = 1) OR (user_id = ?)", userID).Find(&chats, "stream_id = ?", streamID)
	err := query.Error
	if err != nil {
		return nil, err
	}
	for i := range chats {
		normalizeChat(&chats[i], userID)
	}
	return chats, nil
}

// GetAllChats returns all chats for the stream with the given ID
// Number of likes are inserted and the user's like status is determined
func (d chatDao) GetAllChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers").Find(&chats, "stream_id = ?", streamID)
	err := query.Error
	if err != nil {
		return nil, err
	}
	for i := range chats {
		normalizeChat(&chats[i], userID)
	}
	return chats, nil
}

// GetActivePoll returns the active poll for the stream with the given ID.
func (d chatDao) GetActivePoll(streamID uint) (model.Poll, error) {
	var activePoll model.Poll
	err := DB.Preload("PollOptions").First(&activePoll, "stream_id = ? AND active = true", streamID).Error
	return activePoll, err
}

// GetPollUserVote returns the id of the PollOption that the user has voted for. If no vote was found then 0.
func (d chatDao) GetPollUserVote(pollId uint, userId uint) (uint, error) {
	var pollOptionIds []uint
	err := DB.Table("poll_option_user_votes").Select("poll_option_user_votes.poll_option_id").Joins("JOIN chat_poll_options ON chat_poll_options.poll_option_id=poll_option_user_votes.poll_option_id").Where("poll_id = ? AND user_id = ?", pollId, userId).Find(&pollOptionIds).Error
	if err != nil {
		return 0, err
	}

	if len(pollOptionIds) > 0 {
		return pollOptionIds[0], nil
	}
	return 0, nil
}

// GetPollOptionVoteCount returns the vote count of a specific poll-option
func (d chatDao) GetPollOptionVoteCount(pollOptionId uint) (int64, error) {
	var count int64
	err := DB.Table("poll_option_user_votes").Where("poll_option_id = ?", pollOptionId).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ApproveChat sets the attribute 'visible' to true
func (d chatDao) ApproveChat(id uint) error {
	return DB.Model(&model.Chat{}).Where("id = ?", id).Updates(map[string]interface{}{"visible": true}).Error
}

// DeleteChat removes a chat with the given id from the database.
func (d chatDao) DeleteChat(id uint) error {
	return DB.Model(&model.Chat{}).Delete(&model.Chat{}, id).Error
}

// ResolveChat sets the attribute resolved of chat with the given id to true
func (d chatDao) ResolveChat(id uint) error {
	return DB.Model(&model.Chat{}).Where("id = ?", id).Update("resolved", true).Error
}

// ToggleLike adds a like to a message from the user if it doesn't exist, or removes it if it does
func (d chatDao) ToggleLike(userID uint, chatID uint) error {
	err := DB.Exec("INSERT INTO chat_user_likes (user_id, chat_id) VALUES (?, ?)", userID, chatID).Error
	if err == nil {
		return nil // like was added successfully
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 { // 1062: duplicate entry -> message already liked -> remove
		return DB.Exec("DELETE FROM chat_user_likes WHERE user_id = ? AND chat_id = ?", userID, chatID).Error
	}
	return err // some other error
}

// CloseActivePoll closes poll for the stream with the given ID.
func (d chatDao) CloseActivePoll(streamID uint) error {
	return DB.Table("polls").Where("stream_id = ? AND active", streamID).Update("active", false).Error
}

func (d chatDao) GetChatsByUser(userID uint) (chats []model.Chat, err error) {
	return chats, d.db.Find(&chats, "user_id = ?", userID).Error
}

// GetChat returns a chat message with the given id, uses the userId to normalize the chat.
func (d chatDao) GetChat(id uint, userID uint) (*model.Chat, error) {
	var chat model.Chat

	err := d.db.Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers").Find(&chat, "id = ?", id).Error
	if err != nil {
		return &chat, err
	}

	normalizeChat(&chat, userID)
	return &chat, nil
}

func normalizeChat(chat *model.Chat, userID uint) {
	chat.Likes = len(chat.UserLikes)
	for j := range chat.UserLikes {
		if chat.UserLikes[j].ID == userID {
			chat.Liked = true
			break
		}
	}
	chat.AddressedToIds = []uint{}
	for _, user := range chat.AddressedToUsers {
		chat.AddressedToIds = append(chat.AddressedToIds, user.ID)
	}
}
