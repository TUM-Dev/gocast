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
// TODO: Check if sn.stream_id IS NULL is necessary
func (d ingestServerDao) GetBestIngestServer(schoolID uint) (server model.IngestServer, err error) {
	// Case 1: School has own ingest server with free stream slot
	err = DB.Raw(`
        SELECT i.* FROM ingest_servers i
        LEFT JOIN stream_names sn ON i.id = sn.ingest_server_id
        WHERE i.school_id = ? AND sn.stream_id IS NULL
        GROUP BY i.id
        ORDER BY COUNT(sn.stream_id) ASC, i.workload ASC
        LIMIT 1
    `, schoolID).Scan(&server).Error
	if err == nil && server.ID != 0 {
		return server, nil
	}

	// Case 2: School doesn't have own ingest server with free stream slot, but parent school does
	currentSchoolID := schoolID
	for currentSchoolID != 0 {
		var parentSchoolID uint
		err = DB.Table("schools").Where("id = ?", currentSchoolID).Select("parent_id").Row().Scan(&parentSchoolID)
		if err != nil || parentSchoolID == 0 {
			break
		}

		err = DB.Raw(`
            SELECT i.* FROM ingest_servers i
            LEFT JOIN stream_names sn ON i.id = sn.ingest_server_id
            WHERE i.school_id = ? AND sn.stream_id IS NULL
            GROUP BY i.id
            ORDER BY COUNT(sn.stream_id) ASC, i.workload ASC
            LIMIT 1
        `, parentSchoolID).Scan(&server).Error
		if err == nil && server.ID != 0 {
			return server, nil
		}

		currentSchoolID = parentSchoolID
	}

	// Case 3: Fallback to shared ingest server with the least workload and a free stream slot
	err = DB.Raw(`
        SELECT i.* FROM ingest_servers i
        LEFT JOIN stream_names sn ON i.id = sn.ingest_server_id
        WHERE i.shared = true AND sn.stream_id IS NULL
        GROUP BY i.id
        ORDER BY COUNT(sn.stream_id) ASC, i.workload ASC
        LIMIT 1
    `).Scan(&server).Error
	if err != nil {
		return model.IngestServer{}, err
	}

	if server.ID == 0 {
		return model.IngestServer{}, gorm.ErrRecordNotFound
	}

	return server, nil
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
