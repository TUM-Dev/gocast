package dao

import (
	"TUM-Live/model"
	"errors"
	"github.com/go-sql-driver/mysql"
)

func AddMessage(chat *model.Chat) error {
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
	query := DB.Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers")
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
func GetAllChats(userID uint, streamID uint) ([]model.Chat, error) {
	var chats []model.Chat
	query := DB.Preload("Replies").Preload("UserLikes").Preload("AddressedToUsers").Find(&chats, "stream_id = ?", streamID)
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
