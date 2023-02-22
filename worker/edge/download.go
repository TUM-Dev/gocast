package main

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"os/exec"
	"strings"
)

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if jwtPubKey != nil {
		// validate token; every page access requires a valid jwt.
		if !validateToken(w, r, false) {
			return
		}
	}
	w.Header().Add("Content-Type", "video/mp4")
	w.Header().Add("Content-Disposition", "attachment; filename=\"video.mp4\"")
	c := exec.Command("ffmpeg",
		"-i", "http://0.0.0.0"+port+strings.ReplaceAll(r.URL.String(), "download=1", ""),
		"-c", "copy", "-bsf:a", "aac_adtstoasc", "-movflags", "frag_keyframe+empty_moov", "-f", "mp4", "-")
	c.Stdout = w
	err := c.Start()
	if err != nil {
		log.Println(err.Error())
		return
	}
	err = c.Wait()
	if err != nil {
		log.Println(err.Error())
		return
	}
}
