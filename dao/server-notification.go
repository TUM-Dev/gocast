package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"time"
)

//go:generate mockgen -source=server-notification.go -destination ../mock_dao/server-notification.go

type ServerNotificationDao interface {
	CreateServerNotification(ctx context.Context, notification model.ServerNotification) error

	GetCurrentServerNotifications(ctx context.Context) ([]model.ServerNotification, error)
	GetAllServerNotifications(ctx context.Context) ([]model.ServerNotification, error)

	UpdateServerNotification(ctx context.Context, notification model.ServerNotification, id string) error

	DeleteServerNotification(ctx context.Context, notificationId string) error
}

type serverNotificationDao struct {
	db *gorm.DB
}

func NewServerNotificationDao() ServerNotificationDao {
	return serverNotificationDao{db: DB}
}

//CreateServerNotification creates a new ServerNotification
func (d serverNotificationDao) CreateServerNotification(ctx context.Context, notification model.ServerNotification) error {
	err := DB.WithContext(ctx).Create(&notification).Error
	return err
}

//GetCurrentServerNotifications returns all tumlive notifications that are active
func (d serverNotificationDao) GetCurrentServerNotifications(ctx context.Context) ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.WithContext(ctx).Model(&model.ServerNotification{}).Where("start < ? AND expires > ?", time.Now(), time.Now()).Scan(&res).Error
	return res, err
}

//GetAllServerNotifications returns all tumlive notifications
func (d serverNotificationDao) GetAllServerNotifications(ctx context.Context) ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.WithContext(ctx).Find(&res).Error
	return res, err
}

//UpdateServerNotification updates a notification by its id
func (d serverNotificationDao) UpdateServerNotification(ctx context.Context, notification model.ServerNotification, id string) error {
	err := DB.WithContext(ctx).Model(&model.ServerNotification{}).Where("id = ?", id).Updates(notification).Error
	return err
}

//DeleteServerNotification deletes the notification specified by notificationId
func (d serverNotificationDao) DeleteServerNotification(ctx context.Context, notificationId string) error {
	err := DB.WithContext(ctx).Delete(&model.ServerNotification{}, notificationId).Error
	return err
}
