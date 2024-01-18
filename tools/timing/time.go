// Package timing provides time calculation functions used in TUM Live
package timing

import (
	"time"

	"github.com/jinzhu/now"
)

// GetWeeksInYear returns the number of weeks in the given year
func GetWeeksInYear(year int) int {
	now.WeekStartDay = time.Monday
	yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	endOfYear := now.New(yearStart).EndOfYear()
	firstDayOfLastWeek := now.New(endOfYear).BeginningOfWeek()
	y, w := firstDayOfLastWeek.ISOWeek()
	for y != year {
		firstDayOfLastWeek = firstDayOfLastWeek.Add(time.Hour * -24)
		y, w = firstDayOfLastWeek.ISOWeek()
	}
	return w
}
