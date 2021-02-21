package dao

import (
	"TUM-Live-Backend/model"
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
