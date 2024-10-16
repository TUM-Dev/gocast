package model

type Semester struct {
	TeachingTerm string
	Year         int
}

// InRangeOfSemesters checks if s is between firstSemester (inclusive) and lastSemester (inclusive) or is element of semesters slice
func (s *Semester) InRangeOfSemesters(firstSemester Semester, lastSemester Semester, semesters []Semester) bool {
	if semesters == nil {
		if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
			return s.Year == firstSemester.Year && s.TeachingTerm == firstSemester.TeachingTerm
		}
		return s.GreaterEqualThan(firstSemester) && lastSemester.GreaterEqualThan(*s)
	}
	for _, semester := range semesters {
		if s.Year == semester.Year && s.TeachingTerm == semester.TeachingTerm {
			return true
		}
	}
	return false
}

// GreaterEqualThan checks if s comes after or is equal to s1
func (s *Semester) GreaterEqualThan(s1 Semester) bool {
	return s.Year > s1.Year || (s.Year == s1.Year && (s.TeachingTerm == "W" || s1.TeachingTerm == "S"))
}
