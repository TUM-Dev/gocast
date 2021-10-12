package dao

import "TUM-Live/model"

// GetBestIngestServer returns an appropriate ingest server for a stream of the size streamSize.
// we assume that the streamVersion is either "CAM, COMB or PRES" where COMB holds ~3/4 of the viewers and CAM and PRES each one eighth
func GetBestIngestServer() (server model.IngestServer, err error) {
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
