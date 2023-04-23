package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os/exec"
	"strings"
)

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	var jwtClaims *JWTPlaylistClaims
	if jwtPubKey != nil {
		// validate token; every page access requires a valid jwt.
		if claims, ok := validateToken(w, r, true); !ok {
			return
		} else {
			jwtClaims = claims
		}
	}
	if jwtClaims != nil {
		vodsDownloaded.WithLabelValues(jwtClaims.StreamID, jwtClaims.CourseID).Inc()
	}
	w.Header().Add("Content-Type", "video/mp4")
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", jwtClaims.GetFileName()))
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
