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
		"-F", "benutzer="+Cfg.Lrz.Name,
		"-F", "mailadresse="+Cfg.Lrz.Email,
		"-F", "telefon="+Cfg.Lrz.Phone,
		"-F", "unidir=tum",
		"-F", "subdir="+Cfg.Lrz.SubDir,
		"-F", "info=",
		Cfg.Lrz.UploadURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out))
		return err
	}
	return nil
}
