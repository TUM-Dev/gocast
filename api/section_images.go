package api

import (
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
)

// SectionImages is how apiv2 reaches section thumbnail generation. The worker
// fallback and the worker-side delete are only implemented here, against the worker
// connection helpers, so apiv2 is handed this rather than reimplementing them; it goes
// once the workers do.
type SectionImages struct {
	Dao dao.DaoWrapper
	// May be nil, as for v1: then only the workers are asked.
	Manager *runner_manager.Manager
}

// Generate makes the thumbnails for sections, as v1's createVideoSectionBatch does:
// a runner if one takes the job, else a worker. playlistURL must be signed.
func (s SectionImages) Generate(streamID uint, playlistURL string, course model.Course, sections []model.VideoSection) error {
	parameters := generateVideoSectionImagesParameters{
		sections:           sections,
		playlistUrl:        playlistURL,
		courseName:         course.Name,
		courseTeachingTerm: course.TeachingTerm,
		courseYear:         uint32(course.Year),
	}
	return runner_manager.GenerateSectionImages(s.Manager,
		runner_manager.SectionImageRequest{StreamID: streamID, PlaylistURL: playlistURL, Sections: sections},
		func() error { return GenerateVideoSectionImages(s.Dao, &parameters) })
}

// Delete asks a worker to delete a section's thumbnail, as v1's deleteVideoSection does.
func (s SectionImages) Delete(path string) error {
	return DeleteVideoSectionImage(s.Dao.WorkerDao, path)
}
