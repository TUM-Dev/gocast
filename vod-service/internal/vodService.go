package internal

import (
	"fmt"
	"io"
	"log"
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
		log.Fatal("OUTPUT_DIR environment variable not set.")
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
		log.Println(err)
		return
	}
	// write this byte array to our temporary file
	// return that we have successfully uploaded our file!
	fmt.Fprintf(w, "Successfully Uploaded File to %s\n", tempFile.Name())
	go a.packageFile(tempFile.Name(), handler.Filename)
}

var fileNameIllegal = regexp.MustCompile(`[^a-zA-Z0-9_\\.]+`)

func (a *App) packageFile(file, name string) {
	name = fileNameIllegal.ReplaceAllString(name, "_")
	// override eventually existing files
	err := os.RemoveAll(a.config.outputDir + name)
	if err != nil {
		log.Println(err)
		// try to continue anyway
	}
	err = os.MkdirAll(a.config.outputDir+name, os.ModePerm)
	if err != nil {
		log.Println(err)
		return
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
