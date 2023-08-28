package tools

import (
	"errors"
	"github.com/TUM-Dev/gocast/model"
	"log"
	"os/exec"
	"regexp"
	"strings"
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

func MaskEmail(email string) (masked string, err error) {
	mailParts := strings.Split(email, "@")
	if len(mailParts) != 2 {
		return "", errors.New("email doesn't contain @")
	}
	if len(mailParts[0]) == 0 {
		return "", errors.New("email doesn't contain enough characters before @")
	}
	if len(mailParts[1]) < 3 {
		return "", errors.New("email doesn't contain enough characters after @")
	}
	return mailParts[0][0:1] + strings.Repeat("*", len(mailParts[0])-1) + "@" + mailParts[1], nil
}

// MaskLogin masks lrzIds by replacing digits with *
func MaskLogin(login string) (masked string) {
	re := regexp.MustCompile("[0-9]")
	return re.ReplaceAllString(login, "*")
}
