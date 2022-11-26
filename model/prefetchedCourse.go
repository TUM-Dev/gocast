package model

// PrefetchedCourse represents a course we found in tumonline. This course can be found and created through the "create course" user interface
type PrefetchedCourse struct {
	CourseID string `gorm:"column:course_id;type:varchar(255);primaryKey"`
	Name     string `gorm:"column:name;type:varchar(512);index:,class:FULLTEXT;not null;"`
	Year     int    `gorm:"column:year;not null;"`
	Term     string `gorm:"column:term;type:varchar(1);not null;"` // Either W or S
}

// TableName returns the name of the table for the PrefetchedCourse model in the database.
func (*PrefetchedCourse) TableName() string {
	return "prefetched_courses"
}
