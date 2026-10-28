package timing

import (
	"testing"
	"time"
)

func TestGetWeeksInYear(t *testing.T) {
	// example values from https://www.epochconverter.com/weeks/2020
	yw := map[int]int{2020: 53, 2021: 52, 2022: 52, 2023: 52, 2024: 52, 2025: 52, 2026: 53}
	for y, w := range yw {
		if wiy := GetWeeksInYear(y); wiy != w {
			t.Errorf("GetWeeksInYear(%d) = %d, want %d", y, wiy, w)
		}
	}
	for i := 0; i < 2020; i++ {
		if wiy := GetWeeksInYear(i); wiy > 53 || wiy < 52 {
			t.Errorf("GetWeeksInYear(%d) = %d, but must be either 52 or 53", i, wiy)
		}
	}
}

func BenchmarkGetWeeksInYear(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GetWeeksInYear(i)
	}
}

func TestEndOfDay(t *testing.T) {
	loc := time.FixedZone("x", 2*3600)
	got := EndOfDay(time.Date(2026, 3, 5, 10, 30, 0, 0, loc))
	want := time.Date(2026, 3, 5, 23, 59, 59, 999999999, loc)
	if !got.Equal(want) || got.Location() != loc {
		t.Errorf("EndOfDay = %v, want %v in the input's location", got, want)
	}
	if !EndOfDay(time.Date(2026, 3, 5, 0, 0, 0, 0, loc)).Equal(got) {
		t.Error("midnight and midday of the same day must share an end of day")
	}
}
