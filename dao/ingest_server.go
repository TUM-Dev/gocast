package dao

import (
	"time"

	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=ingest_server.go -destination ../mock_dao/ingest_server.go

type IngestServerDao interface {
	SaveSlot(slot model.StreamName)
	SaveIngestServer(server model.IngestServer)

	GetBestIngestServer(schoolID uint) (server model.IngestServer, err error)
	GetTranscodedStreamSlot(ingestServerID uint) (sn model.StreamName, err error)
	GetStreamSlot(ingestServerID uint) (sn model.StreamName, err error)

	RemoveStreamFromSlot(streamID uint) error
}

type ingestServerDao struct {
	db *gorm.DB
}

func NewIngestServerDao() IngestServerDao {
	return ingestServerDao{db: DB}
}

func (d ingestServerDao) SaveSlot(slot model.StreamName) {
	DB.Save(&slot)
}

func (d ingestServerDao) SaveIngestServer(server model.IngestServer) {
	DB.Save(&server)
}

// GetBestIngestServer returns the IngestServer with the least streams assigned to it
// TODO: Include available ingest servers of parent schools
func (d ingestServerDao) GetBestIngestServer(schoolID uint) (server model.IngestServer, err error) {
	if err = DB.Raw(`SELECT i.* FROM stream_names
        JOIN ingest_servers i ON i.id = stream_names.ingest_server_id
        WHERE (stream_id IS NULL AND (i.school_id = ? OR i.shared = true))
        GROUP BY ingest_server_id
        ORDER BY COUNT(ingest_server_id) DESC`, schoolID).Scan(&server).Error; err != nil {
		return
	}
	if err = DB.Where("school_id = ? OR shared = true", schoolID).Order("workload").First(&server).Error; err != nil {
		return
	}
	return
}

func (d ingestServerDao) GetTranscodedStreamSlot(ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.Order("freed_at asc").First(&sn, "is_transcoding AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func (d ingestServerDao) GetStreamSlot(ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.Order("freed_at asc").First(&sn, "is_transcoding = 0 AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func (d ingestServerDao) RemoveStreamFromSlot(streamID uint) error {
	return DB.
		Model(&model.StreamName{}).
		Where("stream_id = ?", streamID).
		Updates(map[string]interface{}{
			"stream_id": gorm.Expr("NULL"),
			"freed_at":  time.Now(),
		}).Error
}
