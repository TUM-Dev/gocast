package dao

import (
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools/timing"
)

// AddStat adds a new statistic entry to the database
func AddStat(stat model.Stat) error {
	return DB.Create(&stat).Error
}

//GetCourseNumStudents returns the number of students enrolled in the course
func GetCourseNumStudents(courseID uint) (int64, error) {
	var res int64
	err := DB.Table("course_users").Where("course_id = ? OR ? = 0", courseID, courseID).Count(&res).Error
	return res, err
}

//GetCourseNumVodViews returns the sum of vod views of a course
func GetCourseNumVodViews(courseID uint) (int, error) {
	var res int
	err := DB.Raw(`SELECT SUM(stats.viewers) FROM stats
		JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? or ? = 0) AND live = 0`, courseID, courseID).Scan(&res).Error
	return res, err
}

//GetCourseNumLiveViews returns the sum of live views of a course based on the maximum views per lecture
func GetCourseNumLiveViews(courseID uint) (int, error) {
	var res int
	err := DB.Raw(`WITH views_per_stream AS (SELECT MAX(stats.viewers) AS y
		FROM stats
        	JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? OR ? = 0)
            AND stats.live = 1
        GROUP BY stats.stream_id)
		SELECT SUM(y)
			FROM views_per_stream`, courseID, courseID).Scan(&res).Error
	return res, err
}

//GetCourseNumVodViewsPerDay returns the daily amount of vod views for each day
func GetCourseNumVodViewsPerDay(courseID uint) ([]Stat, error) {
	var res []Stat
	err := DB.Raw(`SELECT DATE_FORMAT(stats.time, GET_FORMAT(DATE, 'EUR')) AS x, sum(viewers) AS y
		FROM stats
			JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? OR ? = 0) AND live = 0
		GROUP BY DATE(stats.time);`,
		courseID, courseID).Scan(&res).Error
	return res, err
}

//GetCourseStatsWeekdays returns the days and their sum of vod views of a course
func GetCourseStatsWeekdays(courseID uint) ([]Stat, error) {
	var res []Stat
	err := DB.Raw(`SELECT DAYNAME(stats.time) AS x, SUM(stats.viewers) as y
		FROM stats
			JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? OR ? = 0) AND stats.live = 0
		GROUP BY DAYOFWEEK(stats.time);`,
		courseID, courseID).Scan(&res).Error
	return res, err
}

//GetCourseStatsHourly returns the hours with most vod viewing activity of a course
func GetCourseStatsHourly(courseID uint) ([]Stat, error) {
	var res []Stat
	err := DB.Raw(`SELECT HOUR(stats.time) AS x, SUM(stats.viewers) as y
		FROM stats
			JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? or ? = 0) AND stats.live = 0
		GROUP BY HOUR(stats.time);`,
		courseID, courseID).Scan(&res).Error
	return res, err
}

// GetStudentActivityCourseStats fetches statistics on the activity of the course specified by courseID
// if courseID is 0, stats for all courses are fetched. live specifies whether to get live or vod stats.
func GetStudentActivityCourseStats(courseID uint, live bool) ([]Stat, error) {
	var res []struct {
		Year  uint
		Week  uint
		Count int
	}
	countMethod := "MAX" // livestream viewers are the peak viewers of a livestream
	if !live {
		countMethod = "SUM" // vod views are summed up
	}
	err := DB.Raw(`SELECT year(stats.time) AS year, week(stats.time) AS week, `+countMethod+`(stats.viewers) AS count
		FROM stats
        	JOIN streams s ON s.id = stats.stream_id
		WHERE (s.course_id = ? OR ? = 0) AND stats.live = ? AND week(stats.time) > 0
		GROUP BY year, week
		ORDER BY year, week;`, //or ? = 0 -> if courseID is 0, all stats are selected
		courseID, courseID, live).Scan(&res).Error

	var retVal []Stat
	// fill gaps between weeks with 0 values
	var lastWeek uint
	var lastYear uint
	for i, week := range res {
		if i != 0 {
			// Fill gaps between weeks within a year
			if week.Week > lastWeek+1 && int(week.Week) != timing.GetWeeksInYear(int(week.Year)) {
				for j := lastWeek + 1; j < week.Week; j++ {
					retVal = append(retVal, Stat{X: fmt.Sprintf("%d %d", week.Year, j), Y: 0})
				}
			}
			// fill gap until end of year
			if lastYear != week.Year && int(lastWeek) != timing.GetWeeksInYear(int(lastYear)) {
				for j := lastWeek + 1; int(j) <= timing.GetWeeksInYear(int(lastYear)); j++ {
					retVal = append(retVal, Stat{X: fmt.Sprintf("%d %d", lastYear, j), Y: 0})
				}
			}
		}
		retVal = append(retVal, Stat{X: fmt.Sprintf("%d %02d", week.Year, week.Week), Y: week.Count})
		lastWeek = week.Week
		lastYear = week.Year
	}
	return retVal, err
}

//Stat key value struct that is parsable by Chart.js without further modifications.
//See https://www.chartjs.org/docs/master/general/data-structures.html
type Stat struct {
	X string `json:"x"` // label for stat
	Y int    `json:"y"` // value for stat
}
