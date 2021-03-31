package dao

import (
	"TUM-Live/model"
)

func AddMessage(chat model.Chat) {
	DB.Save(&chat)
}
