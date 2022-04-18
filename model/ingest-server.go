package model

import "gorm.io/gorm"

// IngestServer represents a tumlive we ingest our streams to. This is used for load balancing.
type IngestServer struct {
	gorm.Model  `json:"gorm_model"`
	Url         string       `json:"url"`                // e.g. rtmp://user:password@ingest1.huge.tumlive.com
	OutUrl      string       `gorm:"not null"`           // e.g. https://out.server.com/streams/%s/playlist.m3u8 where %s is the stream name
	Workload    int          `json:"workload,omitempty"` // # of streams currently ingesting to this tumlive
	StreamNames []StreamName // array of stream names that will be assigned to this tumlive
}
