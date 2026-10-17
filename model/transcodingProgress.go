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

	// Failed marks a conversion that ended without a video, with Error saying why in
	// words fit for the person who uploaded it. The row stays until the next upload of
	// this version replaces it, so that they get to see it.
	Failed bool   `gorm:"not null; default:false" json:"failed"`
	Error  string `json:"error,omitempty"`
}
