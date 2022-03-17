package dao

import (
	"TUM-Live/model"
	"errors"
	"github.com/go-sql-driver/mysql"
)

func AddChatPollOptionVote(pollOptionId uint, userId uint) error {
	return DB.Exec("INSERT INTO poll_option_user_votes (poll_option_id, user_id) VALUES (?, ?)", pollOptionId, userId).Error
}

func AddChatPoll(poll *model.Poll) error {
	return DB.Save(poll).Error
}

func AddMessage(chat *model.Chat) error {
	return DB.Save(chat).Error
}

// ApproveChat sets the attribute 'visible' to true
func ApproveChat(id uint) error {
	return DB.Model(&model.Chat{}).Where("id = ?", id).Updates(map[string]interface{}{"visible": true}).Error
}

// DeleteChat removes a chat with the given id from the database.
func DeleteChat(id uint) error {
	return DB.Model(&model.Chat{}).Delete(&model.Chat{}, id).Error
}

// ResolveChat sets the attribute resolved of chat with the given id to true
func ResolveChat(id uint) error {
	return DB.Model(&model.Chat{}).Where("id = ?", id).Update("resolved", true).Error
}

// ToggleLike adds a like to a message from the user if it doesn't exist, or removes it if it does
func ToggleLike(userID uint, chatID uint) error {
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

// GetNumLikes returns the number of likes for a message
func GetNumLikes(chatID uint) (int64, error) {
	var numLikes int64
	err := DB.Table("chat_user_likes").Where("chat_id = ?", chatID).Count(&numLikes).Error
	return numLikes, err
}

// GetVisibleChats returns all visible chats for the stream with the given ID
// or sent by user with id 'userID'
// Number of likes are inserted and the user's like status is determined
func GetVisibleChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.Preload("Replies").Preload("UserLikes")
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
	}
	return chats, nil
}

// GetAllChats returns all chats for the stream with the given ID
// Number of likes are inserted and the user's like status is determined
func GetAllChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.Preload("Replies").Preload("UserLikes").Find(&chats, "stream_id = ?", streamID)
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
	}
	return chats, nil
}

// GetActivePoll returns the active poll for the stream with the given ID.
func GetActivePoll(streamID uint) (model.Poll, error) {
	var activePoll model.Poll
	err := DB.Preload("PollOptions").First(&activePoll, "stream_id = ? AND active = true", streamID).Error
	return activePoll, err
}

// GetActivePoll returns the id of the PollOption that the user has voted for. If not vote was found then 0.
func GetPollUserVote(pollId uint, userId uint) (uint, error) {
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
func GetPollOptionVoteCount(pollOptionId uint) (int64, error) {
	var count int64
	err := DB.Table("poll_option_user_votes").Where("poll_option_id = ?", pollOptionId).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
