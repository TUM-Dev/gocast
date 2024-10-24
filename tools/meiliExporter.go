package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/TUM-Dev/gocast/model"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/asticode/go-astisub"
	"github.com/meilisearch/meilisearch-go"
)

type MeiliStream struct {
	ID           uint   `json:"ID"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CourseName   string `json:"courseName"`
	Year         int    `json:"year"`
	TeachingTerm string `json:"semester"`
	CourseID     uint   `json:"courseID"`
	Private      uint   `json:"private"`
	Visibility   string `json:"visibility"` // corresponds to the visibility of the course
}

type MeiliSubtitles struct {
	ID        string `json:"ID"` // meili id: streamID + timestamp
	StreamID  uint   `json:"streamID"`
	Timestamp int64  `json:"timestamp"`
	TextPrev  string `json:"textPrev"` // the previous subtitle line
	Text      string `json:"text"`
	TextNext  string `json:"textNext"` // the next subtitle line
}

type MeiliCourse struct {
	ID           uint   `json:"ID"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Year         int    `json:"year"`
	TeachingTerm string `json:"semester"`
	Visibility   string `json:"visibility"`
}

type MeiliExporter struct {
	c *meilisearch.Client
	d dao.DaoWrapper
}

func NewMeiliExporter(d dao.DaoWrapper) *MeiliExporter {
	c, err := Cfg.GetMeiliClient()
	if err != nil && errors.Is(err, ErrMeiliNotConfigured) {
		return nil
	} else if err != nil {
		logger.Error("could not get meili client", "err", err)
		return nil
	}

	return &MeiliExporter{c, d}
}

// Export exports all relevant search data to MeiliSearch Instance
func (m *MeiliExporter) Export() {
	if m == nil {
		return
	}
	index := m.c.Index("STREAMS")
	_, err := index.DeleteAllDocuments()
	if err != nil {
		logger.Warn("could not delete all old streams", "err", err)
	}
	_, err = m.c.Index("SUBTITLES").DeleteAllDocuments()
	if err != nil {
		logger.Warn("could not delete all old subtitles", "err", err)
	}

	m.d.StreamsDao.ExecAllStreamsWithCoursesAndSubtitlesBatched(func(streams []dao.StreamWithCourseAndSubtitles) {
		meilistreams := make([]MeiliStream, len(streams))
		for i, stream := range streams {
			meilistreams[i] = MeiliStream{
				ID:           stream.ID,
				CourseID:     stream.CourseID,
				Name:         stream.Name,
				Description:  stream.Description,
				CourseName:   stream.CourseName,
				Year:         stream.Year,
				TeachingTerm: stream.TeachingTerm,
				Visibility:   stream.Visibility,
				Private:      stream.Private,
			}
			if stream.Subtitles != "" {
				meiliSubtitles := make([]MeiliSubtitles, 0)

				vtt, err := astisub.ReadFromWebVTT(strings.NewReader(stream.Subtitles))
				if err != nil {
					logger.Warn("could not parse subtitles", "err", err)
					continue
				}
				for i := range vtt.Items {
					sub := MeiliSubtitles{
						ID:        fmt.Sprintf("%d-%d", stream.ID, vtt.Items[i].StartAt.Milliseconds()),
						StreamID:  stream.ID,
						Timestamp: vtt.Items[i].StartAt.Milliseconds(),
						Text:      vtt.Items[i].String(),
					}
					if i > 0 {
						sub.TextPrev = meiliSubtitles[i-1].Text
						meiliSubtitles[i-1].TextNext = sub.Text
					}

					meiliSubtitles = append(meiliSubtitles, sub)
				}

				if len(meiliSubtitles) > 0 {
					_, err := m.c.Index("SUBTITLES").AddDocuments(&meiliSubtitles, "ID")
					if err != nil {
						logger.Error("issue adding subtitles to meili", "err", err)
					}
				}
			}
		}
		_, err := index.AddDocuments(&meilistreams, "ID")
		if err != nil {
			logger.Error("issue adding documents to meili", "err", err)
		}
	})

	coursesIndex := m.c.Index("COURSES")
	_, err = coursesIndex.DeleteAllDocuments()
	if err != nil {
		logger.Warn("could not delete all old courses", "err", err)
	}

	m.d.CoursesDao.ExecAllCourses(func(courses []dao.Course) {
		meilicourses := make([]MeiliCourse, len(courses))
		for i, course := range courses {
			meilicourses[i] = MeiliCourse{
				ID:           course.ID,
				Name:         course.Name,
				Slug:         course.Slug,
				Year:         course.Year,
				TeachingTerm: course.TeachingTerm,
				Visibility:   course.Visibility,
			}
		}
		_, err := coursesIndex.AddDocumentsInBatches(meilicourses, 500, "ID")
		if err != nil {
			logger.Error("issue adding courses to meili", "err", err)
		}
	})
}

func (m *MeiliExporter) SetIndexSettings() {
	if m == nil {
		return
	}
	index := m.c.Index("STREAMS")
	synonyms := map[string][]string{
		"W": {"Wintersemester", "Winter", "WS", "WiSe"},
		"S": {"Sommersemester", "Sommer", "SS", "SoSe", "Summer"},
	}
	_, err := m.c.Index("STREAMS").UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"courseID", "year", "semester", "visibility", "private"},
		SearchableAttributes: []string{"name", "description"},
	})
	if err != nil {
		logger.Warn("could not set settings for meili index STREAMS", "err", err)
	}
	_, err = index.UpdateSynonyms(&synonyms)
	if err != nil {
		logger.Error("could not set synonyms for meili index STREAMS", "err", err)
	}

	_, err = m.c.Index("SUBTITLES").UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"streamID", "courseID"},
		SearchableAttributes: []string{"text"},
		SortableAttributes:   []string{"timestamp"},
	})
	if err != nil {
		logger.Warn("could not set settings for meili index SUBTITLES", "err", err)
	}

	_, err = m.c.Index("COURSES").UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: []string{"ID", "visibility", "year", "semester"},
		SearchableAttributes: []string{"slug", "name"},
		SortableAttributes:   []string{"year", "semester"},
	})
	if err != nil {
		logger.Warn("could not set settings for meili index COURSES", "err", err)
	}
}

// ToMeiliCourses converts slice of model.Course to slice of MeiliCourse
func ToMeiliCourses(cs []model.Course) []MeiliCourse {
	res := make([]MeiliCourse, len(cs))
	for i, c := range cs {
		res[i] = MeiliCourse{
			ID:           c.ID,
			Name:         c.Name,
			Slug:         c.Slug,
			Year:         c.Year,
			TeachingTerm: c.TeachingTerm,
			Visibility:   c.Visibility,
		}
	}
	return res
}

// ToMeiliStreams converts slice of model.Stream to slice of MeiliStream
func ToMeiliStreams(streams []model.Stream, daoWrapper dao.DaoWrapper) ([]MeiliStream, error) {
	res := make([]MeiliStream, len(streams))
	for i, s := range streams {
		c, err := daoWrapper.GetCourseById(context.Background(), s.CourseID)
		if err != nil {
			return nil, err
		}
		var private uint
		if s.Private {
			private = 1
		}

		res[i] = MeiliStream{
			ID:           s.ID,
			Name:         s.Name,
			Description:  s.Description,
			CourseName:   c.Name,
			Year:         c.Year,
			TeachingTerm: c.TeachingTerm,
			CourseID:     s.CourseID,
			Private:      private,
			Visibility:   c.Visibility,
		}
	}
	return res, nil
}
