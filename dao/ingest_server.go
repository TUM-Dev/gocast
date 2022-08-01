package dao

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/model"
	"gorm.io/gorm"
	"time"
)

//go:generate mockgen -source=ingest_server.go -destination ../mock_dao/ingest_server.go

type IngestServerDao interface {
	SaveSlot(ctx context.Context, slot model.StreamName)
	SaveIngestServer(ctx context.Context, server model.IngestServer)

	GetBestIngestServer(ctx context.Context) (server model.IngestServer, err error)
	GetTranscodedStreamSlot(ctx context.Context, ingestServerID uint) (sn model.StreamName, err error)
	GetStreamSlot(ctx context.Context, ingestServerID uint) (sn model.StreamName, err error)

	RemoveStreamFromSlot(ctx context.Context, streamID uint) error
}

type ingestServerDao struct {
	db *gorm.DB
}

func NewIngestServerDao() IngestServerDao {
	return ingestServerDao{db: DB}
}

func (d ingestServerDao) SaveSlot(ctx context.Context, slot model.StreamName) {
	DB.WithContext(ctx).Save(&slot)
}

func (d ingestServerDao) SaveIngestServer(ctx context.Context, server model.IngestServer) {
	DB.WithContext(ctx).Save(&server)
}

// GetBestIngestServer returns the IngestServer with the least streams assigned to it
func (d ingestServerDao) GetBestIngestServer(ctx context.Context) (server model.IngestServer, err error) {
	if err = DB.WithContext(ctx).Raw("SELECT i.* FROM stream_names" +
		" JOIN ingest_servers i ON i.id = stream_names.ingest_server_id" +
		" WHERE stream_id IS NULL" +
		" GROUP BY ingest_server_id" +
		" ORDER BY COUNT(ingest_server_id) DESC").Scan(&server).Error; err != nil {
		return
	}
	if err = DB.WithContext(ctx).Order("workload").First(&server).Error; err != nil {
		return
	}
	return
}

func (d ingestServerDao) GetTranscodedStreamSlot(ctx context.Context, ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.WithContext(ctx).Order("freed_at asc").First(&sn, "is_transcoding AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func (d ingestServerDao) GetStreamSlot(ctx context.Context, ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.WithContext(ctx).Order("freed_at asc").First(&sn, "is_transcoding = 0 AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func (d ingestServerDao) RemoveStreamFromSlot(ctx context.Context, streamID uint) error {
	return DB.WithContext(ctx).
		Model(&model.StreamName{}).
		Where("stream_id = ?", streamID).
		Updates(map[string]interface{}{
			"stream_id": gorm.Expr("NULL"),
			"freed_at":  time.Now(),
		}).Error
}
