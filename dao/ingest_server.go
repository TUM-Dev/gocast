package dao

import (
	"TUM-Live/model"
	"gorm.io/gorm"
)

// GetBestIngestServer returns the ingest-server with the least streams assigned to it
func GetBestIngestServer() (server model.IngestServer, err error) {
	err = DB.Raw("SELECT i.* FROM stream_names" +
		" JOIN ingest_servers i ON i.id = stream_names.ingest_server_id" +
		" WHERE stream_id IS NULL" +
		" GROUP BY ingest_server_id" +
		" ORDER BY COUNT(ingest_server_id) DESC").Scan(&server).Error
	if err = DB.Order("workload").First(&server).Error; err != nil {
		return
	}
	return
}

func GetTranscodedStreamSlot(ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.First(&sn, "is_transcoding AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func GetStreamSlot(ingestServerID uint) (sn model.StreamName, err error) {
	err = DB.First(&sn, "is_transcoding = 0 AND ingest_server_id = ? AND stream_id IS null", ingestServerID).Error
	return
}

func SaveSlot(slot model.StreamName) {
	DB.Save(&slot)
}

func SaveIngestServer(server model.IngestServer) {
	DB.Save(&server)
}

func RemoveStreamFromSlot(streamID uint) error {
	return DB.Model(&model.StreamName{}).Where("stream_id = ?", streamID).Update("stream_id", gorm.Expr("NULL")).Error
}
