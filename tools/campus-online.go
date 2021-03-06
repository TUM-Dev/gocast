package tools

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func FindStudentsForAllCourses(){
	// TODO: get CourseIDs from database, call findStudentsForCourse, save students to db
}

/**
 * scans the CampusOnline API for enrolled students in one course and stores them into the database
 */
func findStudentsForCourse(courseId string) {
	if xmlBytes, err := getXML(fmt.Sprintf("%v/course/students/xml?token=%v&courseID=%v", Cfg.CampusBase, Cfg.CampusToken, courseId)); err != nil {
		log.Printf("Failed to get XML: %v", err)
	} else {
		fmt.Println(string(xmlBytes))
	}
}

func getXML(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, fmt.Errorf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("status error: %v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("read body: %v", err)
	}

	return data, nil
}
