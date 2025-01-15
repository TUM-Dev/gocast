// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

// queries.go provides custom helper functions for querying the database which are not included in the DAO.

import (
	"errors"
	"net/http"
	"time"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

func UpdateUserSettings(db *gorm.DB, user *model.User, req *protobuf.UpdateUserSettingsRequest) (settings []model.UserSetting, err error) {
	userID := user.ID

	// value shouldn't be an empty string if name is changed
	for _, setting := range req.UserSettings {
		if setting.Type == *protobuf.UserSettingType_PREFERRED_NAME.Enum() {
			if setting.Value == "" {
				return nil, e.WithStatus(http.StatusBadRequest, errors.New("preferred name cannot be empty"))
			}
			// check if last name change is at least 3 months ago
			lastChange := model.UserSetting{}
			if err = db.Where("user_id = ? AND type = ?", userID, 1).First(&lastChange).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// no last change found, so we can just continue
			} else {
				diff := time.Since(lastChange.CreatedAt)
				if diff.Hours() < 24*30*3 {
					return nil, e.WithStatus(http.StatusBadRequest, errors.New("preferred name can only be changed every 3 months"))
				}
			}
		}
	}

	for _, setting := range req.UserSettings {
		var userSetting model.UserSetting
		if err = db.Where("user_id = ? AND type = ?", userID, setting.Type+1).First(&userSetting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			userSetting = model.UserSetting{
				UserID: userID,
				Type:   model.UserSettingType(setting.Type + 1),
				Value:  setting.Value,
			}
			if err = db.Create(&userSetting).Error; err != nil {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			}
		} else {
			userSetting.Value = setting.Value

			if err = db.Save(&userSetting).Error; err != nil {
				return nil, e.WithStatus(http.StatusInternalServerError, err)
			}
		}

		settings = append(settings, userSetting)
	}

	return settings, nil
}
