package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// return stream by streaming key
func GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.First(&res, "stream_key = ?", key).Error
	if err != nil { // entry probably not existent -> not authenticated
		fmt.Printf("error getting stream by key: %v\n", err)
		return res, err
	}
	return res, nil
}

func GetStreamByTumOnlineID(ctx context.Context, id uint) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.Preload("Chats").First(&res, "tum_online_event_id = ?", id).Error
	if err != nil {
		return res, err
	}
	return res, nil
}

func GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error) {
	var res model.Stream
	err = DB.Preload("Chats").Preload("Units", func(db *gorm.DB) *gorm.DB {
		return db.Order("unit_start asc")
	}).First(&res, "id = ?", id).Error
	if err != nil {
		fmt.Printf("error getting stream by id: %v\n", err)
		return res, err
	}
	return res, nil
}

func DeleteStreamsWithTumID(ids []uint) {
	// transaction for performance
	_ = DB.Transaction(func(tx *gorm.DB) error {
		for i := range ids {
			tx.Where("tum_online_event_id = ?", ids[i]).Delete(&model.Stream{})
		}
		return nil
	})
}

func CreateStream(ctx context.Context, stream model.Stream) (err error) {
	dbErr := DB.Create(stream).Error
	return dbErr
}

func UpdateStream(stream model.Stream) error {
	err := DB.Save(&stream).Error
	return err
}

func SetStreamLive(ctx context.Context, streamKey string, playlistUrl string) (err error) {
	dbErr := DB.Model(&model.Stream{}).
		Where("stream_key = ?", streamKey).
		Update("live_now", true).
		Update("playlist_url", playlistUrl).
		Error
	return dbErr
}

func GetCurrentLive(ctx context.Context) (currentLive []model.Stream, err error) {
	if streams, found := Cache.Get("AllCurrentlyLiveStreams"); found {
		return streams.([]model.Stream), nil
	}
	var streams []model.Stream
	if err := DB.Find(&streams, "live_now = ?", true).Error; err != nil {
		return nil, err
	}
	Cache.SetWithTTL("AllCurrentlyLiveStreams", streams, 1, time.Minute)
	return streams, err
}

func SetStreamNotLive(ctx context.Context, streamKey string) (err error) {
	Cache.Clear() // costs a bit but hey
	dbErr := DB.Model(&model.Stream{}).
		Where("stream_key = ?", streamKey).
		Update("live_now", false).
		Error
	return dbErr
}

func InsertConvertJob(ctx context.Context, job *model.ProcessingJob) {
	if Logger != nil {
		Logger(ctx, "inserting processing job.")
	}
	DB.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(job)
}

func DeleteStream(streamID string) {
	DB.Where("id = ?", streamID).Delete(&model.Stream{})
	Cache.Clear()
}
