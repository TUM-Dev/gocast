package dao

import (
	"TUM-Live/model"
	"context"
)

func CreateVod(ctx context.Context, vod *model.Recording) error {
	if Logger != nil {
		Logger(ctx, "creating vod")
	}
	err := DB.Create(&vod).Error
	return err
}

func GetAllRecordings(ctx context.Context) ([]model.Recording, error) {
	if Logger != nil {
		Logger(ctx, "finding all recordings")
	}
	var recordings []model.Recording
	err := DB.Find(&recordings).Error
	return recordings, err
}

func GetVodByID(ctx context.Context, id string) (model.Recording, error) {
	if Logger != nil {
		Logger(ctx, "finding all recordings")
	}
	var recording model.Recording
	err := DB.Find(&recording, "id = ?", id).Error
	return recording, err
}
