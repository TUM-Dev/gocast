package dao

import (
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"time"
)

//go:generate mockgen -source=server-notification.go -destination ../mock_dao/server-notification.go

var ServerNotification = NewServerNotificationDao()

type ServerNotificationDao interface {
	CreateServerNotification(notification model.ServerNotification) error

	GetCurrentServerNotifications() ([]model.ServerNotification, error)
	GetAllServerNotifications() ([]model.ServerNotification, error)

	UpdateServerNotification(notification model.ServerNotification, id string) error

	DeleteServerNotification(notificationId string) error
}

type serverNotificationDao struct {
	db *gorm.DB
}

func NewServerNotificationDao() ServerNotificationDao {
	return serverNotificationDao{db: DB}
}

//CreateServerNotification creates a new ServerNotification
func (d serverNotificationDao) CreateServerNotification(notification model.ServerNotification) error {
	err := DB.Create(&notification).Error
	return err
}

//GetCurrentServerNotifications returns all tumlive notifications that are active
func (d serverNotificationDao) GetCurrentServerNotifications() ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.Model(&model.ServerNotification{}).Where("start < ? AND expires > ?", time.Now(), time.Now()).Scan(&res).Error
	return res, err
}

//GetAllServerNotifications returns all tumlive notifications
func (d serverNotificationDao) GetAllServerNotifications() ([]model.ServerNotification, error) {
	var res []model.ServerNotification
	err := DB.Find(&res).Error
	return res, err
}

//UpdateServerNotification updates a notification by its id
func (d serverNotificationDao) UpdateServerNotification(notification model.ServerNotification, id string) error {
	err := DB.Model(&model.ServerNotification{}).Where("id = ?", id).Updates(notification).Error
	return err
}

//DeleteServerNotification deletes the notification specified by notificationId
func (d serverNotificationDao) DeleteServerNotification(notificationId string) error {
	err := DB.Delete(&model.ServerNotification{}, notificationId).Error
	return err
}