package dao

import "TUM-Live/model"

// AddNotification adds a new notification to the database
func AddNotification(notification *model.Notification) error {
	return DB.Create(notification).Error
}

// GetNotifications returns all notifications for the specified targets
func GetNotifications(target ...model.NotificationTarget) ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.Where("target IN ?", target).Find(&notifications).Error
	return notifications, err
}
