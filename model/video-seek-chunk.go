package model

type VideoSeekChunk struct {
	ChunkIndex uint `gorm:"primaryKey;autoIncrement:false" json:"chunkIndex"`
	Hits       uint `gorm:"not null" json:"hits"`
	StreamID   uint `gorm:"primaryKey;autoIncrement:false" json:"streamID"`
}
