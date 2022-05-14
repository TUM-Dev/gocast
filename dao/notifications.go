package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=notifications.go -destination ../mock_dao/notifications.go

type NotificationsDao interface {
	AddNotification(notification *model.Notification) error

	GetNotifications(target ...model.NotificationTarget) ([]model.Notification, error)
	GetAllNotifications() ([]model.Notification, error)

	DeleteNotification(id uint) error
}

type notificationsDao struct {
	db *gorm.DB
}

func NewNotificiationsDao() NotificationsDao {
	return notificationsDao{db: DB}
}

// AddNotification adds a new notification to the database
func (d notificationsDao) AddNotification(notification *model.Notification) error {
	return DB.Create(notification).Error
}

// GetNotifications returns all notifications for the specified targets
func (d notificationsDao) GetNotifications(target ...model.NotificationTarget) ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.Where("target IN ?", target).Order("id DESC").Find(&notifications).Error
	return notifications, err
}

func (d notificationsDao) GetAllNotifications() ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.Find(&notifications).Error
	return notifications, err
}

// DeleteNotification deletes a notification from the database
func (d notificationsDao) DeleteNotification(id uint) error {
	return DB.Unscoped().Delete(&model.Notification{}, id).Error
}
