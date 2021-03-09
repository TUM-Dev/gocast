package model

type Student struct {
	ID      string `gorm:"primaryKey"` // currently matrikelnr. as soon as we get a reply from the it service "obfuscatedID"
	LRZID   string
	Courses []*Course `gorm:"many2many:course_students;"` // sql back reference
}
