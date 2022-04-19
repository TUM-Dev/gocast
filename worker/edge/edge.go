// Package main provides a simple edge tumlive that proxies requests for TUM-Live-Worker and caches immutable files.
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	cacheLock   = sync.Mutex{}
	cachedFiles = make(map[string]time.Time)

	inflghtLock = sync.Mutex{}
	inflight    = make(map[string]*sync.Mutex)

	allowedRe = regexp.MustCompile("^/[a-zA-Z0-9]+/([a-zA-Z0-9_]+/)*[a-zA-Z0-9_]+\\.(ts|m3u8)$") // e.g. /vm123/live/stream/1234.ts
	//allowedRe = regexp.MustCompile("^.*$") // e.g. /vm123/live/strean/1234.ts
)

var originPort = "8085"
var originProto = "http://"

var VersionTag = "dev"

func main() {
	log.Println("Starting edge tumlive version " + VersionTag)
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8089"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + originPort
	}
	originPEnv := os.Getenv("ORIGIN_PORT")
	if originPEnv != "" {
		originPort = originPEnv
	}
	originProtoEnv := os.Getenv("ORIGIN_PROTO")
	if originProtoEnv != "" {
		originProto = originProtoEnv
	}
	ServeEdge(port)
}

func ServeEdge(port string) {
	prepare()
	go func() {
		for {
			cleanup()
			time.Sleep(time.Minute * 5)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", edgeHandler)
	log.Fatal(http.ListenAndServe(port, mux))
}

// edgeHandler proxies requests to TUM-Live-Worker (nginx) and caches immutable files.
func edgeHandler(writer http.ResponseWriter, request *http.Request) {
	if !allowedRe.MatchString(request.URL.Path) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("404 - Not Found"))
		return
	}
	urlParts := strings.SplitN(request.URL.Path, "/", 3)

	// proxy m3u8 playlist
	if strings.HasSuffix(request.URL.Path, ".m3u8") { // -> ["", "vm123", "live/stream/1234.ts"]
		request.Host = urlParts[1]
		request.URL.Path = "" // override by proxy
		u, err := url.Parse(fmt.Sprintf("%s%s:%s/%s", originProto, urlParts[1], originPort, urlParts[2]))
		if err != nil {
			log.Println("Could not parse URL: ", err)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.URL.Path, req.URL.RawPath = u.Path, u.Path
			req.RequestURI = u.RequestURI()
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}
		}
		proxy.ServeHTTP(writer, request)
		return
	}
	err := fetchFile(urlParts[1], urlParts[2])
	if err != nil {
		log.Printf("Could not fetch file: %v", err)
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("502 - Bad Gateway"))
		return
	}
	http.ServeFile(writer, request, cacheDir+"/"+urlParts[2])
}

// fetchFile fetches a file from the origin tumlive and persists it in the cache directory.
// if the file is already in the cache, it is not fetched again.
func fetchFile(host, file string) error {
	diskDir := cacheDir + "/" + file
	// check if file is already in cache
	cacheLock.Lock()
	_, ok := cachedFiles[diskDir]
	cacheLock.Unlock()
	if ok {
		return nil
	}

	inflghtLock.Lock()
	if _, ok = inflight[file]; !ok {
		inflight[file] = &sync.Mutex{}
	}
	curLock := inflight[file]
	curLock.Lock()
	inflghtLock.Unlock()
	defer curLock.Unlock()
	defer delete(inflight, file)

	// check if file is already in cache after acquiring lock:
	_, err := os.Stat(diskDir)
	if err == nil {
		return nil // file in cache, can be served
	}
	if !os.IsNotExist(err) {
		return err // Unknown error
	}
	// file not in cache, fetch it
	filePathPts := strings.SplitN(file, ".", 2)
	if len(filePathPts) != 2 {
		return fmt.Errorf("parse file path: %s", file)
	}
	d := filepath.Dir(diskDir)
	err = os.MkdirAll(d, 0755)
	if err != nil {
		return err
	}
	fileResp, err := http.Get(fmt.Sprintf("%s%s:%s/%s", originProto, host, originPort, file))
	if err != nil {
		return err
	}
	defer fileResp.Body.Close()
	f, err := os.Create(diskDir)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.ReadFrom(fileResp.Body)
	if err != nil {
		return err
	}
	cacheLock.Lock()
	cachedFiles[diskDir] = time.Now()
	cacheLock.Unlock()
	return nil
}

const cacheDir = "/tmp/edge"

func cleanup() {
	// find files older than one hour:
	log.Println("Cleaning up cache")
	cacheLock.Lock()
	defer cacheLock.Unlock()
	removed := 0
	for file, timestamp := range cachedFiles {
		if time.Since(timestamp) > time.Minute*10 { // clean up all files older than 10 minutes
			removed++
			err := os.Remove(file)
			if err != nil {
				log.Println("Could not remove file: ", err)
			}
			delete(cachedFiles, file)
		}
	}
	log.Println("Removed ", removed, " files")
}

// prepare clears the cache and creates the cache directory
func prepare() {
	// Empty cache on startup:
	err := os.RemoveAll(cacheDir)
	if err != nil {
		log.Printf("Could not empty cache directory: %v", err)
	}
	err = os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		log.Fatal("Could not create cache directory for edge requests: ", err)
	}
}
