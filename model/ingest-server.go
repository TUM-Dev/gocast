package model

import "gorm.io/gorm"

// IngestServer represents a server we ingest our streams to. This is used for load balancing.
type IngestServer struct {
	gorm.Model  `json:"gorm_model"`
	Url         string       `json:"url"`                // e.g. rtmp://user:password@ingest1.huge.server.com
	Workload    int          `json:"workload,omitempty"` // # of streams currently ingesting to this server
	StreamNames []StreamName // array of stream names that will be assigned to this server
}
