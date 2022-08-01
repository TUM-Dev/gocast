package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=notifications.go -destination ../mock_dao/notifications.go

type NotificationsDao interface {
	AddNotification(ctx context.Context, notification *model.Notification) error

	GetNotifications(ctx context.Context, target ...model.NotificationTarget) ([]model.Notification, error)
	GetAllNotifications(ctx context.Context) ([]model.Notification, error)

	DeleteNotification(ctx context.Context, id uint) error
}

type notificationsDao struct {
	db *gorm.DB
}

func NewNotificiationsDao() NotificationsDao {
	return notificationsDao{db: DB}
}

// AddNotification adds a new notification to the database
func (d notificationsDao) AddNotification(ctx context.Context, notification *model.Notification) error {
	return DB.WithContext(ctx).Create(notification).Error
}

// GetNotifications returns all notifications for the specified targets
func (d notificationsDao) GetNotifications(ctx context.Context, target ...model.NotificationTarget) ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.WithContext(ctx).Where("target IN ?", target).Order("id DESC").Find(&notifications).Error
	return notifications, err
}

func (d notificationsDao) GetAllNotifications(ctx context.Context) ([]model.Notification, error) {
	var notifications []model.Notification
	err := DB.WithContext(ctx).Find(&notifications).Error
	return notifications, err
}

// DeleteNotification deletes a notification from the database
func (d notificationsDao) DeleteNotification(ctx context.Context, id uint) error {
	return DB.WithContext(ctx).Unscoped().Delete(&model.Notification{}, id).Error
}
