package camera

import "github.com/TUM-Dev/gocast/model"

type SonySRG struct {
	addr string
	auth string
}

func (s SonySRG) SetPreset(presetId int) error {
	//TODO implement me
	panic("implement me")
}

func (s SonySRG) TakeSnapshot(outDir string) (filename string, err error) {
	//TODO implement me
	panic("implement me")
}

func (s SonySRG) GetPresets() ([]model.CameraPreset, error) {
	//TODO implement me
	panic("implement me")
}

func NewSonySRG(addr string, auth string) SonySRG {
	return SonySRG{}
}
