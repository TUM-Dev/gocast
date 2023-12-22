// Package services provides functions for fetching data from the database.
package services

import (
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

func FetchBannerAlerts(db *gorm.DB) (alerts []model.ServerNotification, err error) {
	err = db.Where("start < now() AND expires > now()").Find(&alerts).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return alerts, err
}

func FetchUserNotifications(db *gorm.DB, u *model.User) (notifications []model.Notification, err error) {
	targetFilter := getTargetFilter(*u)

	err = db.Where(targetFilter).Find(&notifications).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return notifications, nil
}

// const (
// 	TargetAll      = iota + 1 //TargetAll Is any user, regardless if logged in or not
// 	TargetUser                //TargetUser Are all users that are logged in
// 	TargetStudent             //TargetStudent Are all users that are logged in and are students
// 	TargetLecturer            //TargetLecturer Are all users that are logged in and are lecturers
// 	TargetAdmin               //TargetAdmin Are all users that are logged in and are admins

// )

// 1 = admin
// 2 = Lecturer
// 3 = geneeric
// 4 = student

func getTargetFilter(user model.User) (targetFilter string) {
	switch user.Role {
	case 1:
		targetFilter = "target = 1"
	case 2:
		targetFilter = "target = 2"
	case 3:
		targetFilter = "target = 3"
	case 4:
		targetFilter = "target = 4"
	default:
		targetFilter = "target = 1"
	}
	return targetFilter
}

func PostDeviceToken(db *gorm.DB, u model.User, deviceToken string) (err error) {
	device := model.Device{
		User:        u,
		DeviceToken: deviceToken,
	}

	var count int64
	if err := db.Table("devices").Where("user_id = ? AND device_token = ?", u.ID, deviceToken).Count(&count).Error; err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}
	print("Count = ?", count)
	if count != 0 {
		return e.WithStatus(http.StatusConflict, errors.New("device is already registered"))
	}

	if err = db.Create(&device).Error; err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}

func DeleteDeviceToken(db *gorm.DB, userID uint, deviceToken string) (err error) {
	device := model.Device{}

	if err = db.Where("user_id = ? AND device_token = ?", userID, deviceToken).First(&device).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusInternalServerError, err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return e.WithStatus(http.StatusNotFound, errors.New("device not found"))
	}

	if err = db.Delete(&device).Error; err != nil {
		return e.WithStatus(http.StatusInternalServerError, err)
	}

	return nil
}
