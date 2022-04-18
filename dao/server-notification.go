package dao

import (
	"TUM-Live/model"
	"time"
)

//GetCurrentServerNotifications returns all tumlive notifications that are active
func GetCurrentServerNotifications() ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.Model(&model.ServerNotification{}).Where("start < ? AND expires > ?", time.Now(), time.Now()).Scan(&res).Error
	return res, err
}

//GetAllServerNotifications returns all tumlive notifications
func GetAllServerNotifications() ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.Find(&res).Error
	return res, err
}

//UpdateServerNotification updates a notification by its id
func UpdateServerNotification(notification model.ServerNotification, id string) error {
	err := DB.Model(&model.ServerNotification{}).Where("id = ?", id).Updates(notification).Error
	return err
}

//DeleteServerNotification deletes the notification specified by notificationId
func DeleteServerNotification(notificationId string) error {
	err := DB.Delete(&model.ServerNotification{}, notificationId).Error
	return err
}

//CreateServerNotification creates a new ServerNotification
func CreateServerNotification(notification model.ServerNotification) error {
	err := DB.Create(&notification).Error
	return err
}
