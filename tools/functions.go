package tools

import (
	"TUM-Live/model"
	"log"
	"os/exec"
)

// CourseListContains checks whether courses contain a course with a given courseId
func CourseListContains(courses []model.Course, courseId uint) bool {
	// not terribly efficient, might use a map later, but as every user only has a handful of courses fast enough
	for _, c := range courses {
		if c.ID == courseId {
			return true
		}
	}

	return false
}

func UploadLRZ(file string) error {
	cmd := exec.Command("curl", "-F",
		"filename=@"+file,
		"-F", "benutzer="+Cfg.LrzUser,
		"-F", "mailadresse="+Cfg.LRZMail,
		"-F", "telefon="+Cfg.LRZPhone,
		"-F", "unidir=tum",
		"-F", "subdir="+Cfg.LRZSubDir,
		"-F", "info=",
		Cfg.LRZUploadURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out))
		return err
	}
	return nil
}
