// Package visibility decides who may see a course or a stream.
//
// The rules take domain types and return a decision, so they can be tested without a
// gRPC context. The package has no database access on purpose: a rule that needs a
// query has outgrown being a rule.
package visibility

import (
	"github.com/TUM-Dev/gocast/model"
)

// Listed reports whether a course belongs in a listing, such as the live courses on
// the start page. A nil user is anonymous.
func Listed(user *model.User, course model.Course) bool {
	if !Reachable(user, course) {
		return false
	}

	// Hidden courses are unlisted, not private: still reachable by direct link, but
	// kept out of listings for everyone but their administrators.
	if course.IsHidden() && !user.IsAdminOfCourse(course) {
		return false
	}

	return true
}

// Reachable reports whether user may open a course by slug.
//
// Not model.User.IsEligibleToWatchCourse, which answers almost the same question
// with different rules for signed-in users; unifying them is a behaviour change.
func Reachable(user *model.User, course model.Course) bool {
	if course.IsLoggedIn() && user == nil {
		return false
	}

	if course.IsEnrolled() && !user.IsAllowedToWatchPrivateCourse(course) {
		return false
	}

	return true
}

// StreamVisible reports whether one stream of a course may be shown to user. It
// assumes the course itself has already been cleared.
func StreamVisible(user *model.User, course model.Course, stream model.Stream) bool {
	if stream.Private {
		return user.IsAdminOfCourse(course)
	}

	return true
}

// VisibleStreams returns the streams of a course visible to user, in order.
func VisibleStreams(user *model.User, course model.Course) []model.Stream {
	streams := make([]model.Stream, 0, len(course.Streams))
	for _, stream := range course.Streams {
		if StreamVisible(user, course, stream) {
			streams = append(streams, stream)
		}
	}

	return streams
}
