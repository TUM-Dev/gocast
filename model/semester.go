package model

type Semester struct {
	TeachingTerm string
	Year         int
}

// IsInRangeOfSemesters checks if s is element of semesters slice
func (s *Semester) IsInRangeOfSemesters(semesters []Semester) bool {
	for _, semester := range semesters {
		if s.Year == semester.Year && s.TeachingTerm == semester.TeachingTerm {
			return true
		}
	}
	return false
}

// IsBetweenSemesters checks if s is between firstSemester (inclusive) and lastSemester (inclusive)
func (s *Semester) IsBetweenSemesters(firstSemester Semester, lastSemester Semester) bool {
	if firstSemester.Year == lastSemester.Year && firstSemester.TeachingTerm == lastSemester.TeachingTerm {
		return s.Year == firstSemester.Year && s.TeachingTerm == firstSemester.TeachingTerm
	}
	return s.IsGreaterEqualThan(firstSemester) && lastSemester.IsGreaterEqualThan(*s)
}

// IsEqual checks if s is equal to otherSemester
func (s *Semester) IsEqual(otherSemester Semester) bool {
	return s.Year == otherSemester.Year && s.TeachingTerm == otherSemester.TeachingTerm
}

// IsGreaterEqualThan checks if s comes after or is equal to s1
func (s *Semester) IsGreaterEqualThan(s1 Semester) bool {
	return s.Year > s1.Year || (s.Year == s1.Year && (s.TeachingTerm == "W" || s1.TeachingTerm == "S"))
}
