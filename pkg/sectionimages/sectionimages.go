// Package sectionimages is how apiv2 reaches section thumbnail generation.
package sectionimages

import (
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/runner_manager"
	"github.com/TUM-Dev/gocast/pkg/workergrpc"
)

// SectionImages generates section thumbnails. The worker fallback is only implemented
// here, against the worker connection helpers, so apiv2 is handed this rather than
// reimplementing it; it goes once the workers do.
type SectionImages struct {
	Dao dao.DaoWrapper
	// May be nil, as for v1: then only the workers are asked.
	Manager *runner_manager.Manager
}

// Generate makes the thumbnails for sections, as v1's createVideoSectionBatch does:
// a runner if one takes the job, else a worker. playlistURL must be signed.
func (s SectionImages) Generate(streamID uint, playlistURL string, course model.Course, sections []model.VideoSection) error {
	parameters := workergrpc.SectionImagesParameters{
		Sections:           sections,
		PlaylistURL:        playlistURL,
		CourseName:         course.Name,
		CourseTeachingTerm: course.TeachingTerm,
		CourseYear:         uint32(course.Year),
	}
	return runner_manager.GenerateSectionImages(s.Manager,
		runner_manager.SectionImageRequest{StreamID: streamID, PlaylistURL: playlistURL, Sections: sections},
		func() error { return workergrpc.GenerateVideoSectionImages(s.Dao, &parameters) })
}
