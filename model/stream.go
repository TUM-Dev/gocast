package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/russross/blackfriday/v2"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/now"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

// StreamStatus is the status of a stream (e.g. converting)
type StreamStatus int

const (
	StatusUnknown    StreamStatus = iota + 1 // StatusUnknown is the default status of a stream
	StatusConverting                         // StatusConverting indicates that a worker is currently converting the stream.
	StatusConverted                          // StatusConverted indicates that the stream has been converted.
)

type Stream struct {
	gorm.Model

	Name             string `gorm:"index:,class:FULLTEXT"`
	Description      string `gorm:"type:text;index:,class:FULLTEXT"`
	CourseID         uint
	Start            time.Time `gorm:"not null"`
	End              time.Time `gorm:"not null"`
	RoomName         string
	RoomCode         string
	EventTypeName    string
	TUMOnlineEventID uint
	SeriesIdentifier string `gorm:"default:null"`
	StreamKey        string `gorm:"not null"`
	PlaylistUrl      string
	PlaylistUrlPRES  string
	PlaylistUrlCAM   string
	FilePath         string //deprecated
	LiveNow          bool   `gorm:"not null"`
	Recording        bool
	Premiere         bool `gorm:"default:null"`
	Ended            bool `gorm:"default:null"`
	Chats            []Chat
	Stats            []Stat
	Units            []StreamUnit
	VodViews         uint `gorm:"default:0"` // todo: remove me before next semester
	StartOffset      uint `gorm:"default:null"`
	EndOffset        uint `gorm:"default:null"`
	LectureHallID    uint `gorm:"default:null"`
	Silences         []Silence
	Files            []File `gorm:"foreignKey:StreamID"`
	Paused           bool   `gorm:"default:false"`
	StreamName       string
	Duration         uint32           `gorm:"default:null"`
	StreamWorkers    []Worker         `gorm:"many2many:stream_workers;"`
	StreamProgresses []StreamProgress `gorm:"foreignKey:StreamID"`
	VideoSections    []VideoSection
	StreamStatus     StreamStatus `gorm:"not null;default:1"`
	Private          bool         `gorm:"not null;default:false"`

	Watched bool `gorm:"-"` // Used to determine if stream is watched when loaded for a specific user.
}

// GetStartInSeconds returns the number of seconds until the stream starts (or 0 if it has already started or is a vod)
func (s Stream) GetStartInSeconds() int {
	if s.LiveNow || s.Recording {
		return 0
	}
	return int(time.Until(s.Start).Seconds())
}

func (s Stream) GetName() string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("Lecture: %s", s.Start.Format("Jan 2, 2006"))
}

func (s Stream) IsConverting() bool {
	return s.StreamStatus == StatusConverting
}

// IsDownloadable returns true if the stream is a recording and has at least one file associated with it.
func (s Stream) IsDownloadable() bool {
	return s.Recording && len(s.Files) > 0
}

// IsSelfStream returns whether the stream is a scheduled stream in a lecture hall
func (s Stream) IsSelfStream() bool {
	return s.LectureHallID == 0
}

// IsPast returns whether the stream end time was reached
func (s Stream) IsPast() bool {
	return s.End.Before(time.Now()) || s.Ended
}

// IsComingUp returns whether the stream begins in 30 minutes
func (s Stream) IsComingUp() bool {
	eligibleForWait := s.Start.Before(time.Now().Add(30*time.Minute)) && time.Now().Before(s.End)
	return !s.IsPast() && !s.Recording && !s.LiveNow && eligibleForWait
}

// TimeSlotReached returns whether stream has passed the starting time
func (s Stream) TimeSlotReached() bool {
	// Used to stop displaying the timer when there is less than 1 minute left
	return time.Now().After(s.Start.Add(-time.Minute)) && time.Now().Before(s.End)
}

// IsStartingInOneDay returns whether the stream starts within 1 day
func (s Stream) IsStartingInOneDay() bool {
	return s.Start.After(time.Now().Add(24 * time.Hour))
}

// IsStartingInMoreThanOneDay returns whether the stream starts in at least 2 days
func (s Stream) IsStartingInMoreThanOneDay() bool {
	return s.Start.After(time.Now().Add(48 * time.Hour))
}

// IsPlanned returns whether the stream is planned or not
func (s Stream) IsPlanned() bool {
	return !s.Recording && !s.LiveNow && !s.IsPast() && !s.IsComingUp()
}

type silence struct {
	Start uint `json:"start"`
	End   uint `json:"end"`
}

func (s Stream) GetSilencesJson() string {
	forServe := make([]silence, len(s.Silences))
	for i := range forServe {
		forServe[i] = silence{
			Start: s.Silences[i].Start,
			End:   s.Silences[i].End,
		}
	}
	if m, err := json.Marshal(forServe); err == nil {
		return string(m)
	}
	return "[]"
}

func (s Stream) GetDescriptionHTML() string {
	unsafe := blackfriday.Run([]byte(s.Description))
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		SanitizeBytes(unsafe)
	return string(html)
}

func (s Stream) FriendlyDate() string {
	return s.Start.Format("Mon 02.01.2006")
}

func (s Stream) FriendlyTime() string {
	return s.Start.Format("02.01.2006 15:04") + " - " + s.End.Format("15:04")
}

func (s Stream) FriendlyNextDate() string {
	if now.With(s.Start).EndOfDay() == now.EndOfDay() {
		return fmt.Sprintf("Today, %02d:%02d", s.Start.Hour(), s.Start.Minute())
	}
	if now.With(s.Start).EndOfDay() == now.With(time.Now().Add(time.Hour*24)).EndOfDay() {
		return fmt.Sprintf("Tomorrow, %02d:%02d", s.Start.Hour(), s.Start.Minute())
	}
	return s.Start.Format("Mon, January 02. 15:04")
}

// Color returns the ui color of the stream that indicates it's status
func (s Stream) Color() string {
	if s.Recording {
		if s.Private {
			return "gray-500"
		}
		return "success"
	} else if s.LiveNow {
		return "danger"
	} else if s.IsPast() {
		return "warn"
	} else {
		return "info"
	}
}

func (s Stream) getJson(lhs []LectureHall, course Course) gin.H {
	var files []gin.H
	for _, file := range s.Files {
		files = append(files, gin.H{
			"id":           file.ID,
			"fileType":     file.Type,
			"friendlyName": file.GetFriendlyFileName(),
		})
	}
	lhName := "Selfstreaming"
	for _, lh := range lhs {
		if lh.ID == s.LectureHallID {
			lhName = lh.Name
			break
		}
	}

	return gin.H{
		"lectureId":        s.Model.ID,
		"courseId":         s.CourseID,
		"seriesIdentifier": s.SeriesIdentifier,
		"name":             s.Name,
		"description":      s.Description,
		"lectureHallId":    s.LectureHallID,
		"lectureHallName":  lhName,
		"streamKey":        s.StreamKey,
		"isLiveNow":        s.LiveNow,
		"isRecording":      s.Recording,
		"isConverting":     s.StreamStatus == StatusConverting,
		"isPast":           s.IsPast(),
		"hasStats":         s.Stats != nil,
		"files":            files,
		"color":            s.Color(),
		"start":            s.Start,
		"end":              s.End,
		"courseSlug":       course.Slug,
		"private":          s.Private,
	}
}

func (s Stream) Attachments() []File {
	attachments := make([]File, 0)
	for _, f := range s.Files {
		if f.Type == FILETYPE_ATTACHMENT {
			attachments = append(attachments, f)
		}
	}
	return attachments
}

var ErrUnbalancedTags = errors.New("unbalanced tags")
var TagExpr = regexp.MustCompile("<(/?)([A-Za-z0-9]+).*?>")
var EntityExpr = regexp.MustCompile("&#?[A-Za-z0-9]+;")

// Based on: https://github.com/mborgerson/GoTruncateHtml
// truncateHtml will truncate a given byte slice to a maximum of maxlen visible
// characters and optionally append ellipsis. HTML tags are automatically closed
// generating valid truncated HTML.
func truncateHtml(buf []byte, maxlen int, ellipsis string) ([]byte, error) {
	// Here's the gist: Scan the input bytestream. While scanning, count the
	// number of visible characters--that is, characters which are not part of
	// markup tags. When a start tag is encountered, push the tag name onto a
	// stack. When visible character count >= maxlen, or the EOF is reached,
	// stop counting. Copy from the input stream the bytes from the start to the
	// current scanning pointer. Finally, pop each tag off the tag stack and
	// append it to the output stream in the form of a closing tag.

	// We will consider HTML or XHTML as valid input. The following elements,
	// called "Void Elements" need not conform to the XHTML <tag /> convention
	// of void elements and may appear simply as <tag>. Hence, if one of the
	// following is picked up by the tag expression as a start tag, do not add
	// it to the stack of tags that should be closed.
	voidElementTags := []string{"area", "base", "br", "col", "embed", "hr",
		"img", "input", "keygen", "link", "meta",
		"param", "source", "track", "wbr"}

	// Check to see if no input was provided.
	if len(buf) == 0 || maxlen == 0 {
		return []byte{}, nil
	}

	var tagStack []string
	visible := 0
	bufPtr := 0

	for bufPtr < len(buf) && visible < maxlen {

		// Move to nearest tag and count visible characters along the way.
		offset := 0
		visibleCharacterMaxReached := false
		entityDetected := false

		for localOffset, runeValue := range string(buf[bufPtr:]) {
			offset = localOffset

			if runeValue == '<' {
				// Start of tag.
				break
			} else if runeValue == '&' {
				// Possible start of HTML Entity
				loc := EntityExpr.FindIndex(buf[bufPtr+localOffset:])
				if loc != nil && loc[0] == 0 {
					// Entity found!
					entityDetected = true
					offset += loc[1] - 1 // Now pointing to ;
				}
				visible += 1
			} else if unicode.IsPrint(runeValue) && !unicode.IsSpace(runeValue) {
				// Printable, non-space character. Increment visible count.
				visible += 1
			}

			// Check if the limit of visible characters has been reached.
			if visible >= maxlen {
				visibleCharacterMaxReached = true
				break
			}

			if entityDetected {
				break
			}
		}

		// Increment bufPtr to end of scanned section
		bufPtr += offset

		// Stop scanning if the end of the buffer was reached or if the max
		// desired visible characters was reached
		if visibleCharacterMaxReached || bufPtr >= len(buf)-1 {
			break
		}

		// If an entity was detected, continue scanning for next tag
		if entityDetected {
			// Advance past the ;
			bufPtr += 1
			continue
		}

		// Now find the expression sub-matches
		matches := TagExpr.FindSubmatch(buf[bufPtr:])
		tagName := string(matches[2])

		// Advance pointer to the end of the tag
		bufPtr += len(matches[0])

		// If this is a void element, do not count it as a start tag
		isVoidElement := false
		for _, voidElementTagName := range voidElementTags {
			if tagName == voidElementTagName {
				isVoidElement = true
				break
			}
		}
		if isVoidElement {
			continue
		}

		isStartTag := len(matches[1]) == 0

		if isStartTag {
			// This is a start tag. Push the tag to the stack.
			tagStack = append(tagStack, tagName)
		} else {
			// This is an end tag. First, check to make sure the end tag is
			// matches what's on top of the stack.
			if len(tagStack) == 0 || tagStack[len(tagStack)-1] != tagName {
				return nil, ErrUnbalancedTags
			}

			// Now, pop the tag stack.
			tagStack = tagStack[0 : len(tagStack)-1]
		}
	}

	// At this point, bufPtr points to the last rune that should be copied to
	// the output stream. Increment bufPtr past this rune, turning bufPtr into
	// the number of bytes that should be copied.
	_, size := utf8.DecodeRune(buf[bufPtr:])
	bufPtr += size

	// Copy the desired input to the output buffer.
	output := buf[0:bufPtr]

	// Copy ellipsis
	output = append(output, []byte(ellipsis)...)

	// Finally, create a closing tag for each tag in the stack.
	for i := len(tagStack) - 1; i >= 0; i-- {
		output = append(output, []byte(fmt.Sprintf("</%s>", tagStack[i]))...)
	}

	return output, nil
}

func (s Stream) TruncatedDescription() string {
	desc := s.GetDescriptionHTML()
	tr, err := truncateHtml([]byte(desc), 150, "...")
	if err != nil {
		_ = []byte("")
	}
	return string(tr)
}
