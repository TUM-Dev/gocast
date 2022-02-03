package dao

import (
	"TUM-Live/model"
	"errors"
	"github.com/go-sql-driver/mysql"
)

func AddMessage(chat *model.Chat) error {
	return DB.Save(chat).Error
}

//ToggleLike adds a like to a message from the user if it doesn't exist, or removes it if it does
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

// GetChats returns all chats for the stream with the given ID. Number of likes are inserted and the user's like status is determined
func GetChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	err := DB.Preload("Replies").Preload("UserLikes").Find(&chats, "stream_id = ?", streamID).Error
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
