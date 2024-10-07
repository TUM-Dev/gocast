package search

// PrefetchedCourse represents a course we found in tumonline. This course can be found and created through the "create course" user interface
type PrefetchedCourse struct {
	CourseID       string `json:"courseID,omitempty"`
	OrganizationID uint   `json:"organizationID,omitempty"`
	Name           string `json:"name,omitempty"`
	Year           int    `json:"year,omitempty"`
	Term           string `json:"term,omitempty"` // Either W or S
}
