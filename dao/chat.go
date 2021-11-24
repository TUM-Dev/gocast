package dao

import (
	"TUM-Live/model"
)

func AddMessage(chat model.Chat) {
	DB.Save(&chat)
}

//IsUserCooledDown returns true if a user sent 5 messages within the last two minutes
func IsUserCooledDown(uid string) (bool, error) {
	var count int64
	err := DB.Table("chats").Where("user_id = ? AND created_at > DATE_SUB(NOW(), INTERVAL 2 MINUTE)", uid).Count(&count).Error
	return count >= 5, err
}
