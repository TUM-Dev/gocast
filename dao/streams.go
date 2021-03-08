package dao

import (
	"TUM-Live/model"
	"context"
	"fmt"
)

// return stream by streaming key
func GetStreamByKey(ctx context.Context, key string) (stream model.Stream, err error) {
	if Logger != nil {
		Logger(ctx, "Getting stream by key from database.")
	}
	var res model.Stream
	err = DB.First(&res, "stream_key = ?", key).Error
	if err != nil { // entry probably not existent -> not authenticated
		fmt.Printf("error getting stream by key: %v\n", err)
		return res, err
	}
	return res, nil
}

func GetStreamByID(ctx context.Context, id string) (stream model.Stream, err error) {
	if Logger != nil {
		Logger(ctx, "Getting stream by id from database.")
	}
	var res model.Stream
	err = DB.First(&res, "id = ?", id).Error
	if err != nil {
		fmt.Printf("error getting stream by id: %v\n", err)
		return res, err
	}
	return res, nil
}

func CreateStream(ctx context.Context, stream model.Stream) (err error) {
	dbErr := DB.Create(stream).Error
	return dbErr
}

func SetStreamLive(ctx context.Context, streamKey string, playlistUrl string) (err error) {
	dbErr := DB.Model(&model.Stream{}).
		Where("stream_key = ?", streamKey).
		Update("live_now", true).
		Update("playlist_url", playlistUrl).
		Error
	return dbErr
}

func GetCurrentLive(ctx context.Context, currentLive *[]model.Stream) (err error) {
	res := DB.Find(&currentLive, "live_now = ?", true)
	return res.Error
}

func SetStreamNotLive(ctx context.Context, streamKey string) (err error) {
	dbErr := DB.Model(&model.Stream{}).
		Where("stream_key = ?", streamKey).
		Update("live_now", false).
		Error
	return dbErr
}
