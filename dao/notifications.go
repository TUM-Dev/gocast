package dao

import "github.com/joschahenningsen/TUM-Live/model"

// AddNotification adds a new notification to the database
func AddNotification(notification *model.Notification) error {
	return DB.Create(notification).Error
}

// DeleteNotification deletes a notification from the database
func DeleteNotification(id uint) error {
	return DB.Unscoped().Delete(&model.Notification{}, id).Error
}

// GetNotifications returns all notifications for the specified targets
func GetNotifications(target ...model.NotificationTarget) ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.Where("target IN ?", target).Order("id DESC").Find(&notifications).Error
	return notifications, err
}

func GetAllNotifications() ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.Find(&notifications).Error
	return notifications, err
}
