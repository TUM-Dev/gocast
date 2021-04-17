package dao

import (
	"TUM-Live/model"
	"context"
	"gorm.io/gorm"
)

func UpdateStreamFullAssoc(vod *model.Stream) error {
	err := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&vod).Error
	return err
}

func SaveStream(vod *model.Stream) error {
	err := DB.Save(&vod).Error
	return err
}

func GetAllRecordings(ctx context.Context) ([]model.Stream, error) {
	if Logger != nil {
		Logger(ctx, "finding all recordings")
	}
	var recordings []model.Stream
	err := DB.Find(&recordings, "recording = 1").Error
	return recordings, err
}
