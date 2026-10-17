package tools

import (
	"fmt"
	"strconv"
	"strings"
)

// TestEnvPolicy restricts what a server booted against a copy of the production
// database may do to the outside world. The zero value is production: everything allowed.
type TestEnvPolicy struct {
	Enabled bool
	// streamCourses are the only courses allowed to start a stream while Enabled.
	streamCourses map[uint]struct{}
}

// TestEnv is the process-wide policy, set once at boot from the command line.
var TestEnv TestEnvPolicy

// NewTestEnvPolicy builds the policy from the --test-env and --stream-courses flags.
// Course IDs without --test-env are rejected rather than ignored: a stray flag in a
// production unit file would otherwise look like it restricts streaming and silently not.
func NewTestEnvPolicy(enabled bool, streamCourses string) (TestEnvPolicy, error) {
	p := TestEnvPolicy{Enabled: enabled, streamCourses: map[uint]struct{}{}}
	if strings.TrimSpace(streamCourses) == "" {
		return p, nil
	}
	if !enabled {
		return TestEnvPolicy{}, fmt.Errorf("--stream-courses requires --test-env")
	}
	for _, part := range strings.Split(streamCourses, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 0)
		if err != nil {
			return TestEnvPolicy{}, fmt.Errorf("--stream-courses: invalid course ID %q", part)
		}
		p.streamCourses[uint(id)] = struct{}{}
	}
	return p, nil
}

// AllowsStream reports whether a stream of the given course may be started.
func (p TestEnvPolicy) AllowsStream(courseID uint) bool {
	if !p.Enabled {
		return true
	}
	_, ok := p.streamCourses[courseID]
	return ok
}

// StreamCourseIDs returns the allowlisted course IDs, for the boot log.
func (p TestEnvPolicy) StreamCourseIDs() []uint {
	ids := make([]uint, 0, len(p.streamCourses))
	for id := range p.streamCourses {
		ids = append(ids, id)
	}
	return ids
}
