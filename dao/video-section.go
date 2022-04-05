package dao

import "TUM-Live/model"

func CreateVideoSection(section *model.VideoSection) error {
	return DB.Create(&section).Error
}

func CreateVideoSectionBatch(sections []model.VideoSection) error {
	var err error
	for _, section := range sections {
		err = CreateVideoSection(&section)
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateVideoSection(section *model.VideoSection) error {
	return DB.Updates(&section).Error
}

func DeleteVideoSection(videoSectionID uint) error {
	return DB.Delete(&model.VideoSection{},"id = ?", videoSectionID).Error
}

func GetVideoSectionByStreamID(streamID uint) ([]model.VideoSection, error) {
	var sections []model.VideoSection
	var err error
	err = DB.Find(&sections, "streamID = ?", streamID).Error
	return sections, err
}


