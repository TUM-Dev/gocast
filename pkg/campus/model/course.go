package model

// Course represents a university course from CAMPUSonline.
type Course struct {
	UID             string
	Title           map[string]string // language code → title, e.g. "de" → "Grundlagen der Informatik"
	SemesterKey     string
	CourseTypeKey   string
	OrganisationUID string
	SemesterHours   float64
}
