package runner

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tum-dev/gocast/runner/config"

	cp "github.com/otiai10/copy"
)

// livestreamCleanup runs every minute and checks for files
//   - prefixed .del- in config.Config.SegmentPath and deletes their directories
//     if they are older than two hours
//   - prefixed .keep- in config.Config.SegmentPath and moves them to config.Config.ErrorPath
//     if they are older than two hours
func (r *Runner) livestreamCleanup(ctx context.Context, log *slog.Logger) {
	log.Info("Starting livestream cleanup")
	for {
		remove, backup, err := determineCleanableDirectories(config.Config.SegmentPath)
		if err != nil {
			log.Error("Couldn't determine all directories to clean up", "err", err)
		}
		log.Debug("determined directories for removal/backup", "remove", remove, "backup", backup)
		//continue anyway, there might be actionable items

		for d, t := range remove {
			log.Debug("checking directory marked for deletion", "dir", d, "time", t)
			if !t.Before(time.Now()) {
				continue
			}
			log.Debug("attempting deletion", "dir", d)
			if err := os.RemoveAll(d); err != nil {
				log.Error("Couldn't remove directory", "dir", d, "err", err)
			}
		}

		for d, t := range backup {
			log.Debug("checking directory marked for backup", "dir", d, "time", t)
			if !t.Before(time.Now()) {
				continue
			}
			dstDir := filepath.Join(config.Config.ErrorPath, strings.TrimPrefix(d, config.Config.SegmentPath))
			log.Debug("attempting backup", "src", d, "dst", dstDir)
			if err := cp.Copy(d, dstDir); err != nil {
				log.Error("Couldn't backup directory", "dir", d, "err", err)
				continue
			}
			log.Debug("deleting backed-up directory", "dir", d)
			if err := os.RemoveAll(d); err != nil {
				log.Error("Couldn't remove directory", "dir", d, "err", err)
			}
		}

		select {
		case <-ctx.Done():
			log.Debug("Context done, stopping livestream cleanup")
			return
		case <-time.After(time.Minute):
		}
	}
}

func determineCleanableDirectories(path string) (remove map[string]time.Time, backup map[string]time.Time, err error) {
	remove = make(map[string]time.Time)
	backup = make(map[string]time.Time)
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".del-") {
			t, err := parseUnix(d.Name()[5:])
			if err != nil {
				return err
			}
			remove[filepath.Dir(path)] = t
		} else if strings.HasPrefix(d.Name(), ".keep-") {
			t, err := parseUnix(d.Name()[6:])
			if err != nil {
				return err
			}
			backup[filepath.Dir(path)] = t
		} else if d.IsDir() && path != config.Config.SegmentPath {
			// find empty directories
			dir, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(dir) == 0 {
				remove[path] = time.Now()
			}
		}
		return nil
	})
	return remove, backup, err
}

func parseUnix(t string) (time.Time, error) {
	unixSec, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q :%w", t, err)
	}
	return time.Unix(unixSec, 0), nil
}
