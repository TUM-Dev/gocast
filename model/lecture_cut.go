package model

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"time"
)

// CutRange is a stretch of a recording, in seconds from its start.
type CutRange struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// LectureCut is a cut of a lecture's VOD that the runners are carrying out. It is kept to
// finish the cut once every version is done, because only then does the lecture's data
// that refers to times in the recording, its sections and offsets, move onto the new
// timeline, and a duplicate become visible. It stays open when a version fails, with the
// ranges, so that the failed versions can be cut again like the others.
//
// A lecture has at most one: SourceStreamID is unique, which is also what makes starting a
// cut atomic.
type LectureCut struct {
	// TargetStreamID is the lecture the cut VOD belongs to: the cut one itself when the
	// VOD is replaced, the copy otherwise.
	TargetStreamID uint `gorm:"primaryKey;autoIncrement:false"`
	SourceStreamID uint `gorm:"not null;uniqueIndex"`
	// SourcePrivate is whether the source was private before the cut. A duplicate takes
	// over the source's visibility, and the source is made private, which a second try at
	// finishing the cut must not mistake for the source's own.
	SourcePrivate bool       `gorm:"not null;default:false"`
	Discard       []CutRange `gorm:"serializer:json;type:text;not null"`
	// Token tells this cut from the next one of the same lecture, which has the same target
	// when the VOD is replaced. A handler that read the cut and was slow to finish it must not
	// finish its successor, see FinishLectureCut. Empty on cuts from before it existed.
	Token     string `gorm:"not null;default:''"`
	CreatedAt time.Time
}

// NewCutToken is a random value for LectureCut.Token.
func NewCutToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Duplicate tells whether the cut made a copy of the lecture.
func (c LectureCut) Duplicate() bool { return c.TargetStreamID != c.SourceStreamID }

// Waveform is the content of the file of type FILETYPE_WAVEFORM.
type Waveform struct {
	// BucketSeconds is the length of what one of the Peaks stands for.
	BucketSeconds uint32 `json:"bucketSeconds"`
	// Peaks is one level per bucket: 0 is -60 dBFS or less, 255 is 0 dBFS.
	Peaks []byte `json:"peaks"`
	// Durations is the length in seconds of each version the runner could measure.
	Durations map[StreamVersion]float64 `json:"durations"`
}

// MaxVersionDurationGap is how much the versions of a stream may differ in length for one
// cut to fit them all.
const MaxVersionDurationGap = 30.0

// DurationGap is the difference between the longest and the shortest version.
func (w Waveform) DurationGap() float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, d := range w.Durations {
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}
	if hi < lo {
		return 0
	}
	return hi - lo
}

// RemapTime is where a time in the recording lies once the discarded ranges are cut out of
// it. A time inside a cut moves to where the cut was. The ranges must not overlap.
func RemapTime(t float64, discard []CutRange) float64 {
	removed := 0.0
	for _, r := range discard {
		removed += math.Max(0, math.Min(r.End, t)-r.Start)
	}
	return math.Max(0, t-removed)
}
