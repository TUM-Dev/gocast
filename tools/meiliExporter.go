package tools

import (
	"errors"
	"fmt"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/asticode/go-astisub"
	"github.com/meilisearch/meilisearch-go"
	"strings"
)

type MeiliStream struct {
	ID           uint   `json:"ID"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CourseName   string `json:"courseName"`
	Year         int    `json:"year"`
	TeachingTerm string `json:"semester"`
	CourseID     uint   `json:"courseID"`
}

type MeiliSubtitles struct {
	ID        string `json:"ID"` // meili id: streamID + timestamp
	StreamID  uint   `json:"streamID"`
	Timestamp int64  `json:"timestamp"`
	TextPrev  string `json:"textPrev"` // the previous subtitle line
	Text      string `json:"text"`
	TextNext  string `json:"textNext"` // the next subtitle line
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

func (m *MeiliExporter) Export() {
	if m == nil {
		return
	}
	index := m.c.Index("STREAMS")
	_, err := m.c.Index("SUBTITLES").DeleteAllDocuments()
	if err != nil {
		logger.Warn("could not delete all old subtitles", "err", err)
	}

	m.d.StreamsDao.ExecAllStreamsWithCoursesAndSubtitles(func(streams []dao.StreamWithCourseAndSubtitles) {
		meilistreams := make([]MeiliStream, len(streams))
		streamIDs := make([]uint, len(streams))
		for i, stream := range streams {
			streamIDs[i] = stream.ID
			meilistreams[i] = MeiliStream{
				ID:           stream.ID,
				CourseID:     stream.CourseID,
				Name:         stream.Name,
				Description:  stream.Description,
				CourseName:   stream.CourseName,
				Year:         stream.Year,
				TeachingTerm: stream.TeachingTerm,
			}
			if stream.Subtitles != "" {
				meiliSubtitles := make([]MeiliSubtitles, 0)

				vtt, err := astisub.ReadFromWebVTT(strings.NewReader(stream.Subtitles))
				if err != nil {
					logger.Warn("could not parse subtitles", "err", err)
					continue
				}
				for i, _ := range vtt.Items {
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
	_, err := index.UpdateSynonyms(&synonyms)
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
}
