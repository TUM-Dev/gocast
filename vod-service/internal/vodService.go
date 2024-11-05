package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type config struct {
	outputDir string
}

type App struct {
	config config
}

func NewApp() *App {
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		logger.Error("OUTPUT_DIR environment variable not set.")
	}
	if !strings.HasSuffix(outputDir, "/") {
		outputDir += "/"
	}
	return &App{config: config{outputDir: outputDir}}
}

func (a *App) Run() {
	http.HandleFunc("/", a.uploadHandler)
	err := http.ListenAndServe(":8089", nil)
	if err != nil {
		fmt.Println(err)
	}
}

func (a *App) uploadHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("got upload request")
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		fmt.Println(err)
		return
	}
	file, handler, err := r.FormFile("filename")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern
	tempFile, err := os.CreateTemp(os.TempDir(), "upload-*"+handler.Filename)
	if err != nil {
		fmt.Println(err)
	}

	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		logger.Error("Error on io copy", "err", err)
		return
	}
	// write this byte array to our temporary file
	// return that we have successfully uploaded our file!
	fmt.Fprintf(w, "Successfully Uploaded File to %s\n", tempFile.Name())
	go a.packageFile(tempFile.Name(), handler.Filename)
}

var fileNameIllegal = regexp.MustCompile(`[^a-zA-Z0-9_\\.]+`)

func (a *App) packageFile(file, name string) {
	defer func() {
		err := os.Remove(file)
		if err != nil {
			logger.Error("Error cleaning up file", "err", err)
		}
	}()
	name = fileNameIllegal.ReplaceAllString(name, "_")
	// override eventually existing files
	err := os.RemoveAll(a.config.outputDir + name)
	if err != nil {
		logger.Error("Error on removing files", "err", err)
		// try to continue anyway
	}
	err = os.MkdirAll(a.config.outputDir+name, os.ModePerm)
	if err != nil {
		logger.Error("Error on creating directories", "err", err)
		return
	}

	cmdCheck := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=nw=1:nk=1", file)
	output, err := cmdCheck.Output()
	if err != nil {
		logger.Error("Error running ffprobe", "err", err)
		return
	}

	codec := strings.TrimSpace(string(output))
	fmt.Printf("Codec: %s\n", codec)

	if codec != "h264" {
		fmt.Println("Converting video to H.264")
		h264File := file + "_h264.mp4"

		cmdConvert := exec.Command("ffmpeg", "-i", file, "-c:v", "libx264", "-preset", "fast", "-c:a", "aac", h264File)
		cmdConvert.Stdout = os.Stdout
		cmdConvert.Stderr = os.Stderr
		err = cmdConvert.Run()
		if err != nil {
			logger.Error("Error converting video to h264", "err", err)
			return
		}

		file = h264File
	}

	c := exec.Command("ffmpeg",
		strings.Split(
			"-i "+file+
				" -c copy "+
				"-f hls "+
				"-hls_time 8 "+
				"-hls_playlist_type vod "+
				"-hls_flags independent_segments "+
				"-hls_segment_type mpegts "+
				"-hls_segment_filename "+a.config.outputDir+name+"/"+"segment%04d.ts "+
				a.config.outputDir+name+"/"+"playlist.m3u8", " ")...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err = c.Run()
	if err != nil {
		fmt.Println(err)
	}
}
