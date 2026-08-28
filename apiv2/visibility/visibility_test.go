package visibility

import (
	"testing"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
)

// Every visibility level crossed with every kind of caller: the bugs these rules have
// had were in combinations nobody thought to check.

const (
	public   = "public"
	loggedIn = "loggedin"
	enrolled = "enrolled"
	hidden   = "hidden"
)

func course(visibility string) model.Course {
	return model.Course{Model: gorm.Model{ID: 1}, Visibility: visibility, UserID: 99}
}

// The callers, by their relationship to course ID 1.
func callers() map[string]*model.User {
	return map[string]*model.User{
		"anonymous": nil,
		"stranger":  {Model: gorm.Model{ID: 2}, Role: model.StudentType},
		"enrolled": {
			Model:   gorm.Model{ID: 3},
			Role:    model.StudentType,
			Courses: []model.Course{course(enrolled)},
		},
		"course admin": {
			Model:               gorm.Model{ID: 4},
			Role:                model.LecturerType,
			AdministeredCourses: []model.Course{course(enrolled)},
		},
		"owner":      {Model: gorm.Model{ID: 99}, Role: model.LecturerType},
		"site admin": {Model: gorm.Model{ID: 5}, Role: model.AdminType},
	}
}

func TestReachable(t *testing.T) {
	want := map[string]map[string]bool{
		//             anonymous stranger enrolled admin  owner  siteadmin
		public:   {"anonymous": true, "stranger": true, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		hidden:   {"anonymous": true, "stranger": true, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		loggedIn: {"anonymous": false, "stranger": true, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		enrolled: {"anonymous": false, "stranger": false, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
	}

	for visibility, expected := range want {
		for name, user := range callers() {
			if got := Reachable(user, course(visibility)); got != expected[name] {
				t.Errorf("Reachable(%s, %s course) = %v, want %v", name, visibility, got, expected[name])
			}
		}
	}
}

func TestListed(t *testing.T) {
	want := map[string]map[string]bool{
		public:   {"anonymous": true, "stranger": true, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		loggedIn: {"anonymous": false, "stranger": true, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		enrolled: {"anonymous": false, "stranger": false, "enrolled": true, "course admin": true, "owner": true, "site admin": true},
		// The one place the two rules differ.
		hidden: {"anonymous": false, "stranger": false, "enrolled": false, "course admin": true, "owner": true, "site admin": true},
	}

	for visibility, expected := range want {
		for name, user := range callers() {
			if got := Listed(user, course(visibility)); got != expected[name] {
				t.Errorf("Listed(%s, %s course) = %v, want %v", name, visibility, got, expected[name])
			}
		}
	}
}

func TestHiddenIsUnlistedRatherThanPrivate(t *testing.T) {
	// Stated on its own because it is the rule most easily "fixed" into a bug: the
	// two differing on hidden courses is intentional.
	c := course(hidden)
	var anonymous *model.User

	if Listed(anonymous, c) {
		t.Error("a hidden course was listed to an anonymous caller")
	}
	if !Reachable(anonymous, c) {
		t.Error("a hidden course was not reachable by direct link, which is what hidden means")
	}
}

func TestStreamVisible(t *testing.T) {
	c := course(public)
	open := model.Stream{Model: gorm.Model{ID: 10}}
	private := model.Stream{Model: gorm.Model{ID: 11}, Private: true}

	for name, user := range callers() {
		if !StreamVisible(user, c, open) {
			t.Errorf("StreamVisible(%s, open stream) = false, want true", name)
		}
	}

	// A private stream is for whoever administers the course; enrolled is not enough.
	admins := map[string]bool{"course admin": true, "owner": true, "site admin": true}
	for name, user := range callers() {
		want := admins[name]
		if got := StreamVisible(user, c, private); got != want {
			t.Errorf("StreamVisible(%s, private stream) = %v, want %v", name, got, want)
		}
	}
}

func TestVisibleStreamsKeepsOrderAndDropsPrivate(t *testing.T) {
	c := course(public)
	c.Streams = []model.Stream{
		{Model: gorm.Model{ID: 1}},
		{Model: gorm.Model{ID: 2}, Private: true},
		{Model: gorm.Model{ID: 3}},
	}

	var anonymous *model.User
	got := VisibleStreams(anonymous, c)

	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("got streams %v, want 1 and 3 in that order", ids(got))
	}

	// An administrator sees all three, still in order.
	all := VisibleStreams(&model.User{Model: gorm.Model{ID: 99}}, c)
	if len(all) != 3 {
		t.Errorf("owner got %v, want all three", ids(all))
	}
}

func ids(streams []model.Stream) []uint {
	out := make([]uint, len(streams))
	for i, s := range streams {
		out[i] = s.ID
	}
	return out
}
