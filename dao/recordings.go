package dao

import (
	"TUM-Live/model"
	"context"
	"gorm.io/gorm"
)

func UpdateStreamFullAssoc(vod *model.Stream) error {
	defer Cache.Clear()
	err := DB.Session(&gorm.Session{FullSaveAssociations: true}).Updates(&vod).Error
	return err
}

func SetStreamNotLiveById(streamID string) error {
	return DB.Table("streams").Where("id = ?", streamID).Update("live_now", "0").Error
}

func SaveStream(vod *model.Stream) error {
	defer Cache.Clear()
	err := DB.Model(&vod).Updates(model.Stream{
		Name:             vod.Name,
		Description:      vod.Description,
		CourseID:         vod.CourseID,
		Start:            vod.Start,
		End:              vod.End,
		RoomName:         vod.RoomName,
		RoomCode:         vod.RoomCode,
		EventTypeName:    vod.EventTypeName,
		PlaylistUrl:      vod.PlaylistUrl,
		PlaylistUrlPRES:  vod.PlaylistUrlPRES,
		PlaylistUrlCAM:   vod.PlaylistUrlCAM,
		FilePath:         vod.FilePath,
		LiveNow:          vod.LiveNow,
		Recording:        vod.Recording,
		Chats:            vod.Chats,
		Stats:            vod.Stats,
		Units:            vod.Units,
		VodViews:         vod.VodViews,
		StartOffset:      vod.StartOffset,
		EndOffset:        vod.EndOffset,
	}).Error
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
