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
	err = DB.First(&res, "streamKey = ?", key).Error
	if err != nil {
		fmt.Printf("error getting stream by key: %v", err)
	}
	if err != nil {
		return res, err
	}
	return res, nil
}
