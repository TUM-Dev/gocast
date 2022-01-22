package dao

import (
	"TUM-Live/model"
)

func AddMessage(chat model.Chat) error {
	return DB.Save(&chat).Error
}
