// Package main provides a simple edge server that proxies requests for TUM-Live-Runner and caches immutable files.
package main

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	log "log/slog"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cacheLock   = sync.Mutex{}
	cachedFiles = make(map[string]time.Time)

	inflightLock = sync.Mutex{}
	inflight     = make(map[string]*sync.Mutex)

	//                                  /itocm2.cit.tum.de/62169/STREAM_VERSION_COMBINED/playlist.m3u8
	allowedRe = regexp.MustCompile(`^/[a-zA-Z0-9.]+/([a-zA-Z0-9_]+/)*([0-9]+|playlist)\.(ts|m3u8)$`) // e.g. /vm123/live/stream/1234.ts
	//allowedRe = regexp.MustCompile("^.*$") // e.g. /vm123/live/strean/1234.ts
)

var port = ":8089"

var originPort = "8187"
var originProto = "http://"

var VersionTag = "dev"

const CertDirEnv = "CERT_DIR"

var vodPath = "/vod"

// CORS header
var allowedOrigin = "*"

var mainInstance = "http://localhost:8081"

var adminToken = ""

func main() {
	l := log.New(log.NewJSONHandler(os.Stdout, &log.HandlerOptions{
		Level: log.LevelInfo,
	})).With("version", VersionTag)
	log.SetDefault(l)
	log.Info("Starting edge server")
	eport := os.Getenv("PORT")
	if eport != "" {
		port = eport
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	originPEnv := os.Getenv("ORIGIN_PORT")
	if originPEnv != "" {
		originPort = originPEnv
	}
	originProtoEnv := os.Getenv("ORIGIN_PROTO")
	if originProtoEnv != "" {
		originProto = originProtoEnv
	}
	vodPathEnv := os.Getenv("VOD_DIR")
	if vodPathEnv != "" {
		vodPath = vodPathEnv
	}
	allowedOriginEnv := os.Getenv("ALLOWED_ORIGIN")
	if allowedOriginEnv != "" {
		allowedOrigin = allowedOriginEnv
	}
	mainInstanceEnv := os.Getenv("MAIN_INSTANCE")
	if mainInstanceEnv != "" {
		mainInstance = mainInstanceEnv
	}
	adminToken = os.Getenv("ADMIN_TOKEN")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2112", nil)
	}()
	ServeEdge(port)
}

var vodFileServer http.Handler

func ServeEdge(port string) {
	prepare()
	go func() {
		for {
			cleanup()
			time.Sleep(time.Minute * 5)
		}
	}()
	go func() {
		for {
			time.Sleep(time.Second * 2)
			concurrentUsers.Set(float64(usersMap.Len()))
		}
	}()

	mux := http.NewServeMux()
	vodFileServer = http.FileServer(http.Dir(vodPath))
	mux.HandleFunc("/vod/", vodHandler)
	mux.HandleFunc("/", edgeHandler)

	go func() {
		err := http.ListenAndServe(port, mux)
		if err != nil {
			log.With("port", port, "err", err).Error("can't listen")
			os.Exit(1)
		}
	}()
	go handleTLS(mux)

	keepAlive()
}

type JWTPlaylistClaims struct {
	jwt.RegisteredClaims
	UserID   uint
	Playlist string
	Download bool
	StreamID string
	CourseID string
}

func (c *JWTPlaylistClaims) GetFileName() string {
	if c == nil {
		return "video.mp4"
	}
	pts := strings.Split(c.Playlist, "/")
	for _, pt := range pts {
		if strings.HasSuffix(pt, ".mp4") {
			return pt
		}
	}
	return fmt.Sprintf("%s.mp4", c.StreamID)
}

func validateToken(w http.ResponseWriter, r *http.Request, download bool) (claims *JWTPlaylistClaims, ok bool) {
	token := r.URL.Query().Get("jwt")
	if token == "" {
		http.Error(w, "Missing JWT", http.StatusForbidden)
		return nil, false
	}

	if token == adminToken {
		return &JWTPlaylistClaims{
			RegisteredClaims: jwt.RegisteredClaims{},
			UserID:           0,
			Playlist:         "adminplayback",
			StreamID:         "-1",
		}, true
	}

	parsedToken, err := jwt.ParseWithClaims(token, &JWTPlaylistClaims{}, func(token *jwt.Token) (interface{}, error) {
		key := jwtPubKey
		return key, nil
	})
	if err != nil { // e.g. some string that is not an actual jwt or signed with another key
		w.WriteHeader(http.StatusForbidden)
		if parsedToken != nil && !parsedToken.Valid {
			_, _ = w.Write([]byte("Forbidden, invalid token"))
		} else {
			_, _ = w.Write([]byte("Forbidden"))
		}
		return nil, false
	}

	// verify that the claimed path in the token matches the request path:
	allowedPlaylist, err := url.Parse(parsedToken.Claims.(*JWTPlaylistClaims).Playlist)
	if err != nil || (allowedPlaylist.Scheme != "https" && allowedPlaylist.Scheme != "http") {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad Request. Parsing URL in jtw failed."))
		return nil, false
	}

	urlParts := strings.Split(allowedPlaylist.Path, "/")
	allowedPath := "/vod/" + strings.Join(urlParts[2:len(urlParts)-1], "/")
	if !strings.HasPrefix(r.URL.Path, allowedPath+"/") {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden. URL doesn't match claim in jwt. " + allowedPath + " vs " + r.URL.Path))
		return nil, false
	}

	if download && !parsedToken.Claims.(*JWTPlaylistClaims).Download {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden, download not allowed."))
		return claims, false
	}

	return parsedToken.Claims.(*JWTPlaylistClaims), true
}

func vodHandler(w http.ResponseWriter, r *http.Request) {
	d := r.URL.Query().Get("download")
	if d != "" && d != "0" {
		downloadHandler(w, r)
		return
	}
	w.Header().Add("Access-Control-Allow-Origin", allowedOrigin)
	var uid string
	if jwtPubKey != nil {
		// validate token; every page access requires a valid jwt.
		claims, ok := validateToken(w, r, false)
		if !ok {
			return
		}
		if claims.UserID != 0 {
			uid = fmt.Sprintf("%d", claims.UserID)
		}
		// add the jwt to all .ts files in the playlist for subsequent verification
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			playlistsRequested.WithLabelValues(claims.StreamID, claims.CourseID).Inc()
			// map request path to path under `vod_path`
			upath := r.URL.Path
			if !strings.HasPrefix(upath, "/") {
				upath = "/" + upath
				r.URL.Path = upath
			}
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/vod")
			f, err := os.Open(path.Join(vodPath, path.Clean(r.URL.Path)))

			if err != nil {
				err404Playlists.WithLabelValues(claims.StreamID, claims.Playlist).Inc()
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("Not Found"))
				return
			}
			fileContents, err := io.ReadAll(f)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal server error. Can't read file: " + f.Name()))
				return
			}
			lines := strings.Split(string(fileContents), "\n")
			for i, line := range lines {
				if strings.HasSuffix(line, ".ts") {
					lines[i] = line + "?jwt=" + r.URL.Query().Get("jwt")
				}
			}
			resp := strings.Join(lines, "\n")
			_, _ = w.Write([]byte(resp))
			return
		} else if strings.HasSuffix(r.URL.Path, ".ts") {
			chunksRequested.WithLabelValues(claims.StreamID, claims.CourseID).Inc()
		}
	}
	if uid == "" {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		uid = ip
	}
	usersMap.Put(uid, true)
	http.StripPrefix("/vod", vodFileServer).ServeHTTP(w, r)
}

func handleTLS(mux *http.ServeMux) {
	if os.Getenv(CertDirEnv) == "" {
		return
	}

	dir, err := os.ReadDir(os.Getenv(CertDirEnv))
	privkeyName := ""
	fullchainName := ""
	if err != nil {
		log.With("err", err).Warn("[HTTPS] Skipping, could not read cert directory")
	} else {
		for _, entry := range dir {
			if strings.HasSuffix(entry.Name(), "privkey.pem") {
				privkeyName = path.Join(os.Getenv(CertDirEnv), entry.Name())
			}
			if strings.HasSuffix(entry.Name(), "fullchain.pem") {
				fullchainName = path.Join(os.Getenv(CertDirEnv), entry.Name())
			}
		}
	}
	if privkeyName != "" && fullchainName != "" {
		go func() {
			err := http.ListenAndServeTLS(":8443", fullchainName, privkeyName, mux)
			if err != nil {
				log.With("err", err).Error("can't listen and serve tls")
				os.Exit(1)
			}
		}()
	} else {
		log.With("err", err).Warn("[HTTPS] Skipping, could not find privkey.pem or fullchain.pem in cert directory")
	}
}

// edgeHandler proxies requests to TUM-Live-Worker and caches immutable files.
func edgeHandler(writer http.ResponseWriter, request *http.Request) {
	if !allowedRe.MatchString(request.URL.Path) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("404 - Not Found"))
		return
	}

	writer.Header().Add("Access-Control-Allow-Origin", allowedOrigin)

	urlParts := strings.SplitN(request.URL.Path, "/", 3) // -> ["", "vm123", "live/stream/1234.ts"]

	// proxy m3u8 playlist
	if strings.HasSuffix(request.URL.Path, ".m3u8") {
		request.Host = urlParts[1]
		request.URL.Path = "" // override by proxy
		u, err := url.Parse(fmt.Sprintf("%s%s:%s/%s", originProto, urlParts[1], originPort, urlParts[2]))
		if err != nil {
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
		log.With("url", urlParts, "err", err).Warn("Could not fetch file")
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

	inflightLock.Lock()
	if _, ok = inflight[file]; !ok {
		inflight[file] = &sync.Mutex{}
	}
	curLock := inflight[file]
	curLock.Lock()
	inflightLock.Unlock()
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
	log.Info("Cleaning up cache")
	cacheLock.Lock()
	defer cacheLock.Unlock()
	removed := 0
	for file, timestamp := range cachedFiles {
		if time.Since(timestamp) > time.Minute*10 { // clean up all files older than 10 minutes
			removed++
			err := os.Remove(file)
			if err != nil {
				log.With("err", err).Error("Could not remove file")
			}
			delete(cachedFiles, file)
		}
	}
	log.With("nFiles ", removed).Info("Removed files")
	err := deleteEmptyDirs(cacheDir)
	if err != nil {
		log.With("err", err).Error("Could not remove empty directories in cache")
	}
}

func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Read at least one entry
	if err == nil {
		return false, nil // Not empty
	}
	if errors.Is(err, os.ErrClosed) {
		return false, fmt.Errorf("directory closed: %s", dir)
	}
	return true, nil // Empty
}

func deleteEmptyDirs(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		empty, err := isDirEmpty(path)
		if err != nil {
			return err
		}
		if empty {
			// Check if the directory is empty or only contains empty directories
			// We need to check all subdirectories recursively
			// So we use a separate function to handle this
			if err := removeEmptyDirs(path); err != nil {
				return err
			}
		}
		return filepath.SkipDir // Skip the directory's contents
	})
}

func removeEmptyDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(dir, entry.Name())
			empty, err := isDirEmpty(subdir)
			if err != nil {
				return err
			}
			if !empty {
				return nil // Not empty, do not delete
			}
			if err := removeEmptyDirs(subdir); err != nil {
				return err
			}
		} else {
			return nil // Not empty, do not delete
		}
	}

	// If we get here, the directory is empty or only contains empty directories
	return os.Remove(dir)
}

var jwtPubKey *rsa.PublicKey

// prepare clears the cache and creates the cache directory
func prepare() {
	output, err := exec.Command("ffmpeg", "-version").CombinedOutput()
	if err != nil {
		panic(err)
	}
	log.With("version", string(output)).Info("probed ffmpeg")
	// Empty cache on startup:
	err = os.RemoveAll(cacheDir)
	if err != nil {
		log.With("err", err).Error("Could not empty cache directory")
	}
	err = os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		log.With("err", err).Error("Could not create cache directory for edge requests")
		os.Exit(1)
	}
	// prevent defaulting to audio/x-mpegurl:
	err = mime.AddExtensionType(".m3u8", "application/vnd.apple.mpegurl")
	if err != nil {
		log.With("err", err).Error("Can't set mimetype for m3u8")
	}
	retries := 0
	backoff := time.Second
	for retries < 5 { // allow for 5 retries with backoff to reach main instance
		log.With("attempt", retries+1).Info("Trying to get jwt key from main instance.")
		retries++
		backoff *= 2
		time.Sleep(backoff)
		resp, err := http.Get(mainInstance + "/jwtPubKey")
		if err != nil {
			log.With("err", err).Warn("Could not get jwt key from main instance")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.With("status", resp.StatusCode).Warn("Could not get jwt key from main instance")
			continue
		}
		decoder := json.NewDecoder(resp.Body)
		jwtPubKeyTmp := rsa.PublicKey{}
		err = decoder.Decode(&jwtPubKeyTmp)
		if err != nil {
			log.With("err", err).Warn("Could not decode jwt key")
			continue
		}
		jwtPubKey = &jwtPubKeyTmp
		log.With("pubKey", jwtPubKey).Info("successfully gathered public key")
		break
	}
}

func keepAlive() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	s := <-sig
	log.With("signal", s).Info("Terminating on signal")
	os.Exit(1)
}
