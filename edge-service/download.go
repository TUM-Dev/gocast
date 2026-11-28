package main

import (
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// contentDisposition builds an attachment header that survives non-ASCII names
// (course titles contain umlauts) and quotes in the name.
func contentDisposition(name string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		return v
	}
	return "attachment"
}

// maxConcurrentDownloads bounds the remuxes running at once: each one holds a
// lecture-sized temp file until it has been sent, so unbounded downloads can fill the disk
// and fail every other download with it.
const maxConcurrentDownloads = 5

// downloadRetryAfter is what rejected clients are told to wait; a remux takes up to about
// a minute, so by then a slot has usually freed.
const downloadRetryAfter = 60

var downloadSlots = make(chan struct{}, maxConcurrentDownloads)

// rejectDownloadBusy answers 503 with a Retry-After. It is rejected instead of queued: a
// queued request sends no bytes for as long as it waits, which proxies and browsers time
// out and which looks like a hang. The CORS header lets the watch page read the status.
func rejectDownloadBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(downloadRetryAfter))
	http.Error(w, "too many downloads are running, try again in a minute", http.StatusServiceUnavailable)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	// Set before anything can fail, so the page can also read a 403 or 503 from the probe.
	w.Header().Add("Access-Control-Allow-Origin", allowedOrigin)
	var jwtClaims *JWTPlaylistClaims
	if jwtPubKey != nil {
		// validate token; every page access requires a valid jwt.
		if claims, ok := validateToken(w, r, true); !ok {
			return
		} else {
			jwtClaims = claims
		}
	}
	// A probe lets the watch page learn whether a download would be accepted before it
	// navigates to it: a rejected navigation would replace the page with the error. It
	// takes no slot, so it can still race with other downloads; the real request is
	// checked again below.
	if r.URL.Query().Get("probe") != "" {
		if len(downloadSlots) >= cap(downloadSlots) {
			rejectDownloadBusy(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	select {
	case downloadSlots <- struct{}{}:
		defer func() { <-downloadSlots }()
	default:
		rejectDownloadBusy(w)
		return
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
	w.Header().Add("Content-Disposition", contentDisposition(jwtClaims.GetFileName()))
	http.ServeContent(w, r, "", time.Time{}, tmp)
}
