package model

import "time"

// CourseRegistration represents a person's registration for a course in CAMPUSonline.
type CourseRegistration struct {
	UID             string
	CourseUID       string
	CourseGroupUID  string
	PersonUID       string
	GivenName       string
	Surname         string
	MatriculationNr string
	Email           string
	Username        string
	LastModifiedAt  time.Time
}
