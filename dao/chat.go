package dao

import (
	"context"
	"errors"
	"github.com/go-sql-driver/mysql"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=chat.go -destination ../mock_dao/chat.go

type ChatDao interface {
	AddChatPollOptionVote(ctx context.Context, pollOptionId uint, userId uint) error
	AddChatPoll(ctx context.Context, poll *model.Poll) error
	AddMessage(ctx context.Context, chat *model.Chat) error

	GetChatUsers(ctx context.Context, streamid uint) ([]model.User, error)
	GetNumLikes(ctx context.Context, chatID uint) (int64, error)
	GetVisibleChats(ctx context.Context, userID uint, streamID uint) ([]model.Chat, error)
	GetAllChats(ctx context.Context, userID uint, streamID uint) ([]model.Chat, error)
	GetActivePoll(ctx context.Context, streamID uint) (model.Poll, error)
	GetPollUserVote(ctx context.Context, pollId uint, userId uint) (uint, error)
	GetPollOptionVoteCount(ctx context.Context, pollOptionId uint) (int64, error)

	ApproveChat(ctx context.Context, id uint) error
	DeleteChat(ctx context.Context, id uint) error
	ResolveChat(ctx context.Context, id uint) error
	ToggleLike(ctx context.Context, userID uint, chatID uint) error

	CloseActivePoll(ctx context.Context, streamID uint) error

	GetChatsByUser(ctx context.Context, userID uint) ([]model.Chat, error)
}

type chatDao struct {
	db *gorm.DB
}

func NewChatDao() ChatDao {
	return chatDao{db: DB}
}

func (d chatDao) AddChatPollOptionVote(ctx context.Context, pollOptionId uint, userId uint) error {
	return DB.WithContext(ctx).Exec("INSERT INTO poll_option_user_votes (poll_option_id, user_id) VALUES (?, ?)", pollOptionId, userId).Error
}

func (d chatDao) AddChatPoll(ctx context.Context, poll *model.Poll) error {
	return DB.WithContext(ctx).Save(poll).Error
}

func (d chatDao) AddMessage(ctx context.Context, chat *model.Chat) error {
	err := DB.WithContext(ctx).Save(chat).Error
	if err != nil {
		return err
	}
	for _, userId := range chat.AddressedToIds {
		err := DB.WithContext(ctx).Exec("INSERT INTO chat_user_addressedto (chat_id, user_id) VALUES (?, ?)", chat.ID, userId).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (d chatDao) GetChatUsers(ctx context.Context, streamid uint) ([]model.User, error) {
	var users []model.User
	query := DB.WithContext(ctx).Model(&model.User{}).Select("distinct users.*").Joins("join chats c on c.user_id = users.id")
	query.Where("c.stream_id = ? AND c.user_name <> ?", streamid, "Anonymous")
	err := query.Scan(&users).Error
	if users == nil { // If no messages have been sent
		users = []model.User{}
	}
	return users, err
}

// GetNumLikes returns the number of likes for a message
func (d chatDao) GetNumLikes(ctx context.Context, chatID uint) (int64, error) {
	var numLikes int64
	err := DB.WithContext(ctx).Table("chat_user_likes").Where("chat_id = ?", chatID).Count(&numLikes).Error
	return numLikes, err
}

// GetVisibleChats returns all visible chats for the stream with the given ID
// or sent by user with id 'userID'
// Number of likes are inserted and the user's like status is determined
func (d chatDao) GetVisibleChats(ctx context.Context, userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.WithContext(ctx).Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers")
	query.Where("(visible = 1) OR (user_id = ?)", userID).Find(&chats, "stream_id = ?", streamID)
	err := query.Error
	if err != nil {
		return nil, err
	}
	for i := range chats {
		chats[i].Likes = len(chats[i].UserLikes)
		for j := range chats[i].UserLikes {
			if chats[i].UserLikes[j].ID == userID {
				chats[i].Liked = true
				break
			}
		}
		chats[i].AddressedToIds = []uint{}
		for _, user := range chats[i].AddressedToUsers {
			chats[i].AddressedToIds = append(chats[i].AddressedToIds, user.ID)
		}
	}
	return chats, nil
}

// GetAllChats returns all chats for the stream with the given ID
// Number of likes are inserted and the user's like status is determined
func (d chatDao) GetAllChats(ctx context.Context, userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.WithContext(ctx).Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers").Find(&chats, "stream_id = ?", streamID)
	err := query.Error
	if err != nil {
		return nil, err
	}
	for i := range chats {
		chats[i].Likes = len(chats[i].UserLikes)
		for j := range chats[i].UserLikes {
			if chats[i].UserLikes[j].ID == userID {
				chats[i].Liked = true
				break
			}
		}
		chats[i].AddressedToIds = []uint{}
		for _, user := range chats[i].AddressedToUsers {
			chats[i].AddressedToIds = append(chats[i].AddressedToIds, user.ID)
		}
	}
	return chats, nil
}

// GetActivePoll returns the active poll for the stream with the given ID.
func (d chatDao) GetActivePoll(ctx context.Context, streamID uint) (model.Poll, error) {
	var activePoll model.Poll
	err := DB.WithContext(ctx).Preload("PollOptions").First(&activePoll, "stream_id = ? AND active = true", streamID).Error
	return activePoll, err
}

// GetPollUserVote returns the id of the PollOption that the user has voted for. If no vote was found then 0.
func (d chatDao) GetPollUserVote(ctx context.Context, pollId uint, userId uint) (uint, error) {
	var pollOptionIds []uint
	err := DB.WithContext(ctx).Table("poll_option_user_votes").Select("poll_option_user_votes.poll_option_id").Joins("JOIN chat_poll_options ON chat_poll_options.poll_option_id=poll_option_user_votes.poll_option_id").Where("poll_id = ? AND user_id = ?", pollId, userId).Find(&pollOptionIds).Error
	if err != nil {
		return 0, err
	}

	if len(pollOptionIds) > 0 {
		return pollOptionIds[0], nil
	}
	return 0, nil
}

// GetPollOptionVoteCount returns the vote count of a specific poll-option
func (d chatDao) GetPollOptionVoteCount(ctx context.Context, pollOptionId uint) (int64, error) {
	var count int64
	err := DB.WithContext(ctx).Table("poll_option_user_votes").Where("poll_option_id = ?", pollOptionId).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ApproveChat sets the attribute 'visible' to true
func (d chatDao) ApproveChat(ctx context.Context, id uint) error {
	return DB.WithContext(ctx).Model(&model.Chat{}).Where("id = ?", id).Updates(map[string]interface{}{"visible": true}).Error
}

// DeleteChat removes a chat with the given id from the database.
func (d chatDao) DeleteChat(ctx context.Context, id uint) error {
	return DB.WithContext(ctx).Model(&model.Chat{}).Delete(&model.Chat{}, id).Error
}

// ResolveChat sets the attribute resolved of chat with the given id to true
func (d chatDao) ResolveChat(ctx context.Context, id uint) error {
	return DB.WithContext(ctx).Model(&model.Chat{}).Where("id = ?", id).Update("resolved", true).Error
}

// ToggleLike adds a like to a message from the user if it doesn't exist, or removes it if it does
func (d chatDao) ToggleLike(ctx context.Context, userID uint, chatID uint) error {
	err := DB.WithContext(ctx).Exec("INSERT INTO chat_user_likes (user_id, chat_id) VALUES (?, ?)", userID, chatID).Error
	if err == nil {
		return nil // like was added successfully
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 { // 1062: duplicate entry -> message already liked -> remove
		return DB.WithContext(ctx).Exec("DELETE FROM chat_user_likes WHERE user_id = ? AND chat_id = ?", userID, chatID).Error
	}
	return err // some other error
}

// CloseActivePoll closes poll for the stream with the given ID.
func (d chatDao) CloseActivePoll(ctx context.Context, streamID uint) error {
	return DB.WithContext(ctx).Table("polls").Where("stream_id = ? AND active", streamID).Update("active", false).Error
}

func (d chatDao) GetChatsByUser(ctx context.Context, userID uint) (chats []model.Chat, err error) {
	return chats, d.db.WithContext(ctx).Find(&chats, "user_id = ?", userID).Error
}
