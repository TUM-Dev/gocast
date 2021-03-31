package dao

import (
	"TUM-Live/model"
)

func AddMessage(message string, userId string, name string, vidId uint) {
	msg := model.Chat{
		UserID:   userId,
		Message:  message,
		UserName: name,
		StreamID: vidId,
	}
	DB.Save(&msg)
}
