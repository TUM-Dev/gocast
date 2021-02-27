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

func CreateStream(ctx context.Context, stream model.Stream) (err error) {
	dbErr := DB.Create(stream).Error
	return dbErr
}

func CreateCurrentLive(ctx context.Context, currentLive *model.CurrentLive) (err error) {
	dbErr := DB.Create(currentLive).Error
	return dbErr
}

func GetCurrentLive(ctx context.Context, currentLive *[]model.CurrentLive) (err error) {
	res := DB.Find(&currentLive)
	return res.Error
}

func DeleteCurrentLive(ctx context.Context, key string) (err error) {
	res := DB.Delete(&model.CurrentLive{}, "url = ?", "http://localhost:7002/live/"+key+".m3u8")
	return res.Error
}
