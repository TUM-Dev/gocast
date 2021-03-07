package model

type StudentToCourse struct {
	ObfuscatedID string `gorm:"primaryKey"`
	CourseID     uint   `gorm:"primaryKey"` // Composite primary key prevents duplication of a combination of ObfuscatedID and CourseID
}
