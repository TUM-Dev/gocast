package model

type VideoType string

const (
	VideoTypeCombined     VideoType = "COMB"
	VideoTypePresentation VideoType = "PRES"
	VideoTypeCamera       VideoType = "CAM"
)

func (v VideoType) Valid() bool {
	return v == VideoTypeCombined || v == VideoTypePresentation || v == VideoTypeCamera
}
