package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"bytes"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//FetchCameraPresets Queries all cameras of lecture halls for their camera presets and saves them to the database
func FetchCameraPresets() {
	lectureHalls := dao.GetAllLectureHalls()
	for _, lectureHall := range lectureHalls {
		log.Printf("camera: %s", lectureHall.CameraIP)
		if lectureHall.CameraIP != "" {
			camCurl := exec.Command("curl",
				"--digest", "--user", Cfg.CameraAuthentication,
				"-d", "action=list&group=root.PTZ.Preset.P0.Position.*.Name",
				fmt.Sprintf("http://%s/axis-cgi/param.cgi", lectureHall.CameraIP))
			var bts []byte
			buf := bytes.NewBuffer(bts)
			camCurl.Stdout = buf
			err := camCurl.Start()
			if err != nil {
				log.Printf("%v", err)
				sentry.CaptureException(err)
				continue
			}
			err = camCurl.Wait()
			if err != nil {
				log.Printf("%v", err)
				sentry.CaptureException(err)
				continue
			}
			presets := strings.Split(buf.String(), "\n")
			var presetsForLectureHall []model.CameraPreset
			for _, preset := range presets {
				if presetSplit := strings.Split(preset, "="); len(presetSplit) == 2 {
					idParts := strings.Split(presetSplit[0], ".")
					if len(idParts) != 7 {
						log.Println("Wrong format for camera preset response.")
						sentry.AddBreadcrumb(&sentry.Breadcrumb{Type: "breadcrumb", Data: map[string]interface{}{"parts": idParts}, Level: sentry.LevelDebug, Timestamp: time.Now()})
						sentry.CaptureException(errors.New("wrong format for camera preset response"))
						continue
					}
					presetId, err := strconv.ParseInt(strings.Replace(idParts[len(idParts)-2], "P", "", 1), 10, 0)
					if err != nil {
						sentry.CaptureException(err)
						continue
					}

					log.Printf("id:{%s}, name:{%s}", presetId, presetSplit[1])
					presetsForLectureHall = append(presetsForLectureHall, model.CameraPreset{
						Name:          presetSplit[1],
						PresetID:      int(presetId),
						LectureHallId: lectureHall.Model.ID,
					})
				}
			}
			lectureHall.CameraPresets = presetsForLectureHall
			dao.SaveLectureHallFullAssoc(lectureHall)
		}
	}
}
