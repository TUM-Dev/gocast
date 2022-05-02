package model

type DownloadableFile struct {
	ID   int
	File File `gorm:"embedded"`
}
