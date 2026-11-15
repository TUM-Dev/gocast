package model

import "testing"

func TestRemapTime(t *testing.T) {
	cuts := []CutRange{{10, 20}, {50, 60}}
	for _, tt := range []struct {
		name string
		at   float64
		want float64
	}{
		{"before the first cut", 5, 5},
		{"at the start of a cut", 10, 10},
		{"inside a cut", 15, 10},
		{"at the end of a cut", 20, 10},
		{"between the cuts", 30, 20},
		{"inside the second cut", 55, 40},
		{"after both", 100, 80},
	} {
		if got := RemapTime(tt.at, cuts); got != tt.want {
			t.Errorf("%s: RemapTime(%v) = %v, want %v", tt.name, tt.at, got, tt.want)
		}
	}
	if got := RemapTime(42, nil); got != 42 {
		t.Errorf("without cuts: %v", got)
	}
}

func TestWaveformDurationGap(t *testing.T) {
	for _, tt := range []struct {
		name string
		d    map[StreamVersion]float64
		want float64
	}{
		{"none", nil, 0},
		{"one", map[StreamVersion]float64{COMB: 100}, 0},
		{"two", map[StreamVersion]float64{COMB: 100, CAM: 87.5}, 12.5},
		{"three", map[StreamVersion]float64{COMB: 100, CAM: 130, PRES: 95}, 35},
	} {
		if got := (Waveform{Durations: tt.d}).DurationGap(); got != tt.want {
			t.Errorf("%s: gap = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestLectureCutDuplicate(t *testing.T) {
	if (LectureCut{TargetStreamID: 3, SourceStreamID: 3}).Duplicate() {
		t.Error("a cut that replaces the VOD is no duplicate")
	}
	if !(LectureCut{TargetStreamID: 4, SourceStreamID: 3}).Duplicate() {
		t.Error("a cut into another lecture is a duplicate")
	}
}
