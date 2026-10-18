package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
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

	// ffmpeg can't write a complete moov (duration, seek table) to a pipe, so
	// streaming its output yields a fragmented MP4 that players like VLC stop
	// playing after a few seconds. Remux into a seekable temp file with the moov
	// up front instead, and serve that: it also gives us Content-Length and Range.
	tmp, err := os.CreateTemp("", "download-*.mp4")
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "could not prepare download", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// CommandContext so ffmpeg stops when the client gives up.
	c := exec.CommandContext(r.Context(), "ffmpeg",
		"-y",
		"-fflags", "+genpts", // generating correct time labels for the whole length
		"-i", "http://0.0.0.0"+port+strings.ReplaceAll(r.URL.String(), "download=1", ""),
		"-c", "copy", "-bsf:a", "aac_adtstoasc",
		"-movflags", "+faststart",
		"-f", "mp4", tmp.Name())
	if out, err := c.CombinedOutput(); err != nil {
		// Nothing is sent yet, so the client sees a real error instead of a truncated file.
		log.Printf("download remux failed: %v\n%s", err, out)
		if r.Context().Err() == nil {
			http.Error(w, "could not prepare download", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Add("Content-Type", "video/mp4")
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", jwtClaims.GetFileName()))
	http.ServeContent(w, r, "", time.Time{}, tmp)
}
