package model

type StreamVersion string

const (
	COMB StreamVersion = "COMB"
	CAM  StreamVersion = "CAM"
	PRES StreamVersion = "PRES"
)

// TranscodingProgress is the progress as a percentage of the conversion of a single stream view (e.g. stream 123, COMB view)
type TranscodingProgress struct {
	StreamID uint          `gorm:"primaryKey" json:"streamID"`
	Version  StreamVersion `gorm:"primaryKey" json:"version"`

	Progress int `gorm:"not null; default:0" json:"progress"`
}
