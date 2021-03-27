package dao

import (
	"TUM-Live/model"
)

func AddMessage(message string, userId string, vidId uint) {
	msg := model.Chat{
		UserID:   userId,
		Message:  message,
		StreamID: vidId,
	}
	DB.Save(&msg)
}
