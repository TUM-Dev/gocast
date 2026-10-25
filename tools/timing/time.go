// Package timing provides time calculation functions used in TUM Live
package timing

import "time"

// GetWeeksInYear returns the number of ISO weeks in the given year
func GetWeeksInYear(year int) int {
	// December 28th always falls into the last ISO week of its year.
	_, w := time.Date(year, time.December, 28, 0, 0, 0, 0, time.Local).ISOWeek()
	return w
}

// EndOfDay returns the last representable instant of t's day, in t's location.
func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-1), t.Location())
}
