package runner_manager

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"github.com/TUM-Dev/gocast/model"
)

// camPresQuality is the JPEG quality of the composed thumbnail, as the worker's was.
const camPresQuality = 90

// camPresInset is the gap in pixels between the camera picture and the top edge of the
// composed thumbnail.
const camPresInset = 5

// composeCamPres saves a thumbnail that shows the presentation with the camera picture
// inset in its top right corner, once both of them exist. The worker did this when its
// thumbnails arrived; GetLGThumbnail prefers the result over any single-source thumbnail.
//
// It runs after each CAM or PRES thumbnail is stored, so a regenerated one is composed
// again. A stream with only one of the two is left alone.
//
// Failing to compose is logged and not returned: the notification that led here was
// handled, and the runner would repeat it for nothing.
func (m *Manager) composeCamPres(streamID uint, dir string) {
	logger := m.logger.With("stream", streamID)
	pres, err := m.dao.FileDao.GetThumbnail(streamID, model.FILETYPE_THUMB_LG_PRES)
	if err != nil {
		return // not there yet, or not at all
	}
	cam, err := m.dao.FileDao.GetThumbnail(streamID, model.FILETYPE_THUMB_LG_CAM)
	if err != nil {
		return
	}

	fname := fmt.Sprintf("%d_CAM_PRES.jpeg", streamID)
	dest := filepath.Join(dir, fname)
	if err := combineThumbnails(pres.Path, cam.Path, dest); err != nil {
		logger.Warn("can't compose the camera and presentation thumbnails", "err", err)
		return
	}
	err = m.dao.FileDao.SetThumbnail(streamID, model.File{
		StreamID: streamID,
		Path:     dest,
		Filename: fname,
		Type:     model.FILETYPE_THUMB_LG_CAM_PRES,
	})
	if err != nil {
		logger.Warn("can't save the composed thumbnail", "err", err)
	}
}

// combineThumbnails draws the secondary image at a third of the primary's size into its
// top right corner and writes the result to dest as a JPEG.
func combineThumbnails(primaryPath, secondaryPath, dest string) error {
	primary, err := decodeImage(primaryPath)
	if err != nil {
		return err
	}
	secondary, err := decodeImage(secondaryPath)
	if err != nil {
		return err
	}

	pb := primary.Bounds()
	small := image.NewRGBA(image.Rect(0, 0, pb.Dx()/3, pb.Dy()/3))
	xdraw.CatmullRom.Scale(small, small.Rect, secondary, secondary.Bounds(), xdraw.Over, nil)

	combined := image.NewRGBA(pb)
	draw.Draw(combined, pb, primary, pb.Min, draw.Src)
	at := image.Pt(pb.Max.X-small.Rect.Dx()-camPresInset, pb.Min.Y+camPresInset)
	draw.Draw(combined, small.Rect.Add(at), small, image.Point{}, draw.Over)

	// Written beside the target and renamed, so a reader never sees half a JPEG.
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".campres-*.jpeg")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after the rename
	if err := jpeg.Encode(tmp, combined, &jpeg.Options{Quality: camPresQuality}); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return img, nil
}
