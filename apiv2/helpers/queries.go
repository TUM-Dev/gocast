// Package helpers provides helper functions for parsing models to protobuf representations.
package helpers

// queries.go provides custom helper functions for querying the database which are not included in the DAO.

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

func UpdateUserSettings(db *gorm.DB, user *model.User, req *protobuf.UpdateUserSettingsRequest) (settings []model.UserSetting, err error) {
	userID := user.ID

	// These writes bypass the DAO, so the cached user has to be dropped here or a
	// client reading a setting straight back gets the value from before the write.
	defer dao.InvalidateUserCache(userID)

	for _, setting := range req.UserSettings {
		if setting.Type != *protobuf.UserSettingType_PREFERRED_NAME.Enum() {
			continue
		}

		var lastChange *model.UserSetting
		existing := model.UserSetting{}
		err = db.Where("user_id = ? AND type = ?", userID, model.PreferredName).First(&existing).Error
		switch {
		case err == nil:
			lastChange = &existing
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Never set, so there is no previous change to rate-limit against.
		default:
			return nil, e.WithStatus(http.StatusInternalServerError, err)
		}

		if err := ValidatePreferredName(setting.Value, lastChange, time.Now()); err != nil {
			return nil, err
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

// preferredNameCooldown is how long a user must wait between name changes.
const preferredNameCooldown = 24 * 30 * 3 * time.Hour

// ValidatePreferredName reports whether a user may set this preferred name now.
//
// lastChange is their current setting, or nil if they have never set one. The clock is
// a parameter so the cooldown can be tested without waiting three months.
//
// The cooldown runs from UpdatedAt, not CreatedAt: gorm's Save leaves CreatedAt at the
// first write, so measuring from it would rate-limit only the second change ever.
func ValidatePreferredName(value string, lastChange *model.UserSetting, now time.Time) error {
	if value == "" {
		return e.WithStatus(http.StatusBadRequest, errors.New("preferred name cannot be empty"))
	}

	if len(value) > model.MaxUsernameLength {
		return e.WithStatus(http.StatusBadRequest, fmt.Errorf(
			"preferred name cannot be longer than %d characters", model.MaxUsernameLength))
	}

	if lastChange != nil && now.Sub(lastChange.UpdatedAt) < preferredNameCooldown {
		return e.WithStatus(http.StatusBadRequest,
			errors.New("preferred name can only be changed every 3 months"))
	}

	return nil
}
