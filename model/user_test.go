package model

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

// Course visibilities used across the access control tests. They are the four values
// the Visibility column may hold; a typo in one of them would silently make a course
// behave as "enrolled", which is why they are spelled once here.
var (
	publicCourse   = Course{Model: gorm.Model{ID: 1}, Visibility: "public", UserID: 100}
	loggedInCourse = Course{Model: gorm.Model{ID: 2}, Visibility: "loggedin", UserID: 100}
	enrolledCourse = Course{Model: gorm.Model{ID: 3}, Visibility: "enrolled", UserID: 100}
	hiddenCourse   = Course{Model: gorm.Model{ID: 4}, Visibility: "hidden", UserID: 100}
)

func student(id uint, enrolledIn ...Course) *User {
	return &User{Model: gorm.Model{ID: id}, Role: StudentType, Courses: enrolledIn}
}

// IsEligibleToWatchCourse decides whether a stream may be played at all, so every
// visibility is pinned against every kind of caller including the anonymous one.
func TestIsEligibleToWatchCourse(t *testing.T) {
	admin := &User{Model: gorm.Model{ID: 1}, Role: AdminType}
	owner := &User{Model: gorm.Model{ID: 100}, Role: LecturerType}
	granted := &User{Model: gorm.Model{ID: 2}, Role: LecturerType, AdministeredCourses: []Course{enrolledCourse}}
	otherLecturer := &User{Model: gorm.Model{ID: 3}, Role: LecturerType, AdministeredCourses: []Course{publicCourse}}

	tests := []struct {
		name   string
		user   *User
		course Course
		want   bool
	}{
		{"an anonymous caller may watch a public course", nil, publicCourse, true},
		// Hidden means unlisted, not private: an unauthenticated caller holding the
		// link is deliberately still allowed in.
		{"an anonymous caller may watch a hidden course", nil, hiddenCourse, true},
		{"an anonymous caller may not watch a loggedin course", nil, loggedInCourse, false},
		{"an anonymous caller may not watch an enrolled course", nil, enrolledCourse, false},

		{"any signed in user may watch a loggedin course", student(10), loggedInCourse, true},
		{"a student not enrolled may not watch an enrolled course", student(10), enrolledCourse, false},
		{"a student enrolled may watch an enrolled course", student(10, enrolledCourse), enrolledCourse, true},
		// The enrolment list is matched by course ID, so an enrolment in some other
		// course must not leak access.
		{"a student enrolled elsewhere may not watch an enrolled course", student(10, publicCourse), enrolledCourse, false},

		{"an admin may watch an enrolled course they are not in", admin, enrolledCourse, true},
		{"the owner may watch their own enrolled course", owner, enrolledCourse, true},
		{"a lecturer granted the course may watch it", granted, enrolledCourse, true},
		{"a lecturer granted a different course may not", otherLecturer, enrolledCourse, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.IsEligibleToWatchCourse(tt.course); got != tt.want {
				t.Errorf("IsEligibleToWatchCourse = %v, want %v", got, tt.want)
			}
		})
	}
}

// IsAllowedToWatchPrivateCourse is the gate used for courses that are not public; it
// must never widen to an anonymous caller, not even for a public or hidden course.
func TestIsAllowedToWatchPrivateCourse(t *testing.T) {
	tests := []struct {
		name   string
		user   *User
		course Course
		want   bool
	}{
		// Unlike IsEligibleToWatchCourse this returns false for nil before it ever
		// looks at the visibility.
		{"an anonymous caller is refused even for a public course", nil, publicCourse, false},
		{"an anonymous caller is refused even for a hidden course", nil, hiddenCourse, false},
		{"an enrolled student is allowed", student(10, enrolledCourse), enrolledCourse, true},
		{"an unenrolled student is refused", student(10), enrolledCourse, false},
		{"a signed in user is allowed a loggedin course", student(10), loggedInCourse, true},
		{"an admin is allowed", &User{Model: gorm.Model{ID: 1}, Role: AdminType}, enrolledCourse, true},
		{"the owner is allowed", &User{Model: gorm.Model{ID: 100}, Role: LecturerType}, enrolledCourse, true},
		{
			"a lecturer granted the course is allowed",
			&User{Model: gorm.Model{ID: 2}, Role: LecturerType, AdministeredCourses: []Course{enrolledCourse}},
			enrolledCourse, true,
		},
		{
			"a lecturer granted a different course is refused",
			&User{Model: gorm.Model{ID: 3}, Role: LecturerType, AdministeredCourses: []Course{publicCourse}},
			enrolledCourse, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.IsAllowedToWatchPrivateCourse(tt.course); got != tt.want {
				t.Errorf("IsAllowedToWatchPrivateCourse = %v, want %v", got, tt.want)
			}
		})
	}
}

// Search results are the one place a hidden course must not surface: being able to
// watch it with the link does not mean it may be listed.
func TestIsEligibleToSearchForCourse(t *testing.T) {
	tests := []struct {
		name   string
		user   *User
		course Course
		want   bool
	}{
		{"an anonymous caller finds a public course", nil, publicCourse, true},
		{"an anonymous caller does not find a hidden course", nil, hiddenCourse, false},
		{"a signed in user does not find a hidden course", student(10), hiddenCourse, false},
		// Enrolment alone does not unhide it; only administration does.
		{"a student enrolled in a hidden course still does not find it", student(10, hiddenCourse), hiddenCourse, false},
		{"an admin finds a hidden course", &User{Model: gorm.Model{ID: 1}, Role: AdminType}, hiddenCourse, true},
		{"the owner finds their own hidden course", &User{Model: gorm.Model{ID: 100}, Role: LecturerType}, hiddenCourse, true},
		{"an enrolled student finds an enrolled course", student(10, enrolledCourse), enrolledCourse, true},
		{"an unenrolled student does not find an enrolled course", student(10), enrolledCourse, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.IsEligibleToSearchForCourse(tt.course); got != tt.want {
				t.Errorf("IsEligibleToSearchForCourse = %v, want %v", got, tt.want)
			}
		})
	}
}

// Every access check must tolerate the anonymous caller: these are reached from
// handlers before authentication is established.
func TestAccessChecksDoNotPanicOnNilUser(t *testing.T) {
	var u *User
	for _, c := range []Course{publicCourse, loggedInCourse, enrolledCourse, hiddenCourse} {
		u.IsEligibleToWatchCourse(c)
		u.IsAllowedToWatchPrivateCourse(c)
		u.IsEligibleToSearchForCourse(c)
		if u.IsAdminOfCourse(c) {
			t.Errorf("nil user administers course %d", c.ID)
		}
	}
}

// A password round trip is the login path; the wrong-password and empty-hash cases are
// what keep a broken comparison from admitting everyone.
func TestSetPasswordAndCompare(t *testing.T) {
	t.Run("a correct password matches", func(t *testing.T) {
		u := &User{}
		if err := u.SetPassword("hunter2hunter2"); err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		match, err := u.ComparePasswordAndHash("hunter2hunter2")
		if err != nil {
			t.Fatalf("ComparePasswordAndHash: %v", err)
		}
		if !match {
			t.Error("the password that was set does not match its own hash")
		}
	})

	t.Run("a wrong password does not match", func(t *testing.T) {
		u := &User{}
		if err := u.SetPassword("hunter2hunter2"); err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
		match, err := u.ComparePasswordAndHash("hunter2hunter3")
		if err != nil {
			t.Fatalf("ComparePasswordAndHash: %v", err)
		}
		if match {
			t.Error("a wrong password matched")
		}
	})

	t.Run("a short password is rejected", func(t *testing.T) {
		u := &User{Password: "untouched"}
		if err := u.SetPassword("1234567"); err == nil {
			t.Error("a 7 character password was accepted")
		}
		if u.Password != "untouched" {
			t.Error("a rejected password still overwrote the stored hash")
		}
	})

	// An account with no password (the usual case: everyone who signs in via SSO)
	// must not be loggable into with the empty string.
	t.Run("a user without a password never matches", func(t *testing.T) {
		u := &User{}
		for _, attempt := range []string{"", "anything"} {
			match, err := u.ComparePasswordAndHash(attempt)
			if err != nil || match {
				t.Errorf("ComparePasswordAndHash(%q) = %v, %v; want false, nil", attempt, match, err)
			}
		}
	})

	// A hash column corrupted by a bad migration must surface as an error rather than
	// as a successful login.
	t.Run("a malformed stored hash reports an error", func(t *testing.T) {
		u := &User{Password: "not-a-hash"}
		match, err := u.ComparePasswordAndHash("whatever")
		if err == nil {
			t.Error("a malformed stored hash did not report an error")
		}
		if match {
			t.Error("a malformed stored hash matched")
		}
	})
}

// If the salt were ever fixed, two users with the same password would share a hash and
// the whole table would be crackable at once.
func TestGenerateFromPasswordSaltsEachHash(t *testing.T) {
	first, err := GenerateFromPassword("hunter2hunter2")
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	second, err := GenerateFromPassword("hunter2hunter2")
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	if first == second {
		t.Error("two hashes of the same password are identical, so the salt is not random")
	}

	// The encoding is parsed back by decodeHash and by other argon2 tooling, so its
	// shape is part of the contract.
	if !strings.HasPrefix(first, "$argon2id$v=") {
		t.Errorf("unexpected hash encoding: %q", first)
	}
	if n := len(strings.Split(first, "$")); n != 6 {
		t.Errorf("hash has %d $-separated fields, want 6", n)
	}
}

// decodeHash parses attacker-adjacent data (whatever sits in the password column), so
// every malformed shape must come back as an error instead of a panic or a false match.
func TestDecodeHashRejectsMalformedInput(t *testing.T) {
	valid, err := GenerateFromPassword("hunter2hunter2")
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	tests := []struct {
		name    string
		encoded string
		wantErr error // nil means "any error"
	}{
		{"an empty string", "", ErrInvalidHash},
		{"a string with no separators", "argon2id", ErrInvalidHash},
		{"a truncated hash missing the derived key", strings.Join(strings.Split(valid, "$")[:5], "$"), ErrInvalidHash},
		{"a hash with an extra field", valid + "$extra", ErrInvalidHash},
		{"an unparsable version field", "$argon2id$version$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0$aGFzaA", nil},
		{"an older argon2 version", "$argon2id$v=18$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0$aGFzaA", ErrIncompatibleVersion},
		{"an unparsable parameter field", "$argon2id$v=19$m=lots,t=3,p=2$c2FsdHNhbHRzYWx0$aGFzaA", nil},
		{"a salt that is not base64", "$argon2id$v=19$m=65536,t=3,p=2$not base64!$aGFzaA", nil},
		{"a derived key that is not base64", "$argon2id$v=19$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0$not base64!", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, salt, hash, err := decodeHash(tt.encoded)
			if err == nil {
				t.Fatalf("decodeHash(%q) accepted the input", tt.encoded)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("decodeHash error = %v, want %v", err, tt.wantErr)
			}
			if salt != nil || hash != nil {
				t.Error("decodeHash returned material alongside an error")
			}
		})
	}

	t.Run("a hash generated here round trips", func(t *testing.T) {
		_, salt, hash, err := decodeHash(valid)
		if err != nil {
			t.Fatalf("decodeHash: %v", err)
		}
		if len(salt) == 0 || len(hash) == 0 {
			t.Fatalf("decodeHash returned salt=%d bytes, hash=%d bytes", len(salt), len(hash))
		}
	})
}

// decodeHash used to write the parameters it parsed into the package level `p`, which
// both downgraded the cost of every password hashed afterwards by the process and raced
// between concurrent logins. The parsed parameters are per-hash state now.
func TestDecodeHashLeavesGlobalParametersUntouched(t *testing.T) {
	before := p

	// Same argon2 version, but half the memory of the configured parameters.
	weak := "$argon2id$v=19$m=32768,t=1,p=1$c2FsdHNhbHRzYWx0$aGFzaGhhc2hoYXNo"
	params, _, _, err := decodeHash(weak)
	if err != nil {
		t.Fatalf("decodeHash: %v", err)
	}
	if p != before {
		t.Errorf("global params = %+v, want %+v; decodeHash mutated the generation policy", p, before)
	}
	if params.memory != 32768 || params.iterations != 1 || params.parallelism != 1 {
		t.Errorf("decoded params = %+v; the hash's own parameters were not returned", params)
	}
}

// Stored hashes predate any change to the generation policy, so verification has to use
// each hash's own parameters. A fix that reached for the global `p` instead would lock
// out every user whose hash was made with different ones.
func TestComparePasswordAndHashUsesTheStoredParameters(t *testing.T) {
	const password = "hunter2hunter2"

	// Deliberately weaker than the current policy, as a legacy hash would be.
	legacy := argonParams{memory: 32 * 1024, iterations: 1, parallelism: 1, saltLength: 16, keyLength: 32}
	u := &User{Password: encodeWithParams(t, password, legacy)}

	match, err := u.ComparePasswordAndHash(password)
	if err != nil {
		t.Fatalf("ComparePasswordAndHash: %v", err)
	}
	if !match {
		t.Error("a hash stored with older parameters no longer verifies")
	}

	match, err = u.ComparePasswordAndHash("hunter2hunter3")
	if err != nil {
		t.Fatalf("ComparePasswordAndHash: %v", err)
	}
	if match {
		t.Error("a wrong password matched a hash with older parameters")
	}

	// Hashing a new password afterwards must still use the configured policy.
	fresh, err := GenerateFromPassword(password)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	freshParams, _, _, err := decodeHash(fresh)
	if err != nil {
		t.Fatalf("decodeHash: %v", err)
	}
	if freshParams.memory != p.memory || freshParams.iterations != p.iterations || freshParams.parallelism != p.parallelism {
		t.Errorf("new hash encoded %+v, want the configured policy %+v", freshParams, p)
	}
}

// Concurrent logins all decode at once; under -race this pins the unsynchronised write
// to the global that decodeHash used to perform.
func TestComparePasswordAndHashIsConcurrencySafe(t *testing.T) {
	const password = "hunter2hunter2"
	before := p

	users := []*User{
		{Password: encodeWithParams(t, password, argonParams{memory: 32 * 1024, iterations: 1, parallelism: 1, saltLength: 16, keyLength: 32})},
		{Password: encodeWithParams(t, password, argonParams{memory: 16 * 1024, iterations: 2, parallelism: 4, saltLength: 8, keyLength: 16})},
		{Password: encodeWithParams(t, password, p)},
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, u := range users {
			wg.Add(1)
			go func(u *User) {
				defer wg.Done()
				match, err := u.ComparePasswordAndHash(password)
				if err != nil {
					t.Errorf("ComparePasswordAndHash: %v", err)
					return
				}
				if !match {
					t.Error("a concurrent verification of a correct password failed")
				}
			}(u)
		}
	}
	wg.Wait()

	if p != before {
		t.Errorf("global params = %+v, want %+v; concurrent verification mutated them", p, before)
	}
}

// encodeWithParams builds a stored hash with explicit parameters, standing in for rows
// written before the current policy.
func encodeWithParams(t *testing.T, password string, params argonParams) string {
	t.Helper()
	salt := make([]byte, params.saltLength)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	hash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, params.memory, params.iterations, params.parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
}

// The settings table stores raw strings for some keys and JSON for others; a missing or
// unparsable row must fall back to the documented default rather than blank the UI.
func TestSettingsDefaultsAndFallbacks(t *testing.T) {
	t.Run("a user without settings gets the defaults", func(t *testing.T) {
		u := &User{Name: "Ada"}
		if got := u.GetPreferredName(); got != "Ada" {
			t.Errorf("GetPreferredName = %q, want the account name", got)
		}
		if got := u.GetPreferredGreeting(); got != "Moin" {
			t.Errorf("GetPreferredGreeting = %q, want %q", got, "Moin")
		}
		if got := u.GetPreferredView(); got != "Combined" {
			t.Errorf("GetPreferredView = %q, want %q", got, "Combined")
		}
		if got := u.GetSeekingTime(); got != 10 {
			t.Errorf("GetSeekingTime = %d, want 10", got)
		}
		if !u.PreferredNameChangeAllowed() {
			t.Error("a user who never set a preferred name may not change it")
		}
	})

	t.Run("a set value is returned verbatim", func(t *testing.T) {
		u := &User{Name: "Ada", Settings: []UserSetting{
			{Type: PreferredName, Value: "Ada L."},
			{Type: Greeting, Value: "Hi"},
			{Type: LectureView, Value: "Presentation"},
			{Type: SeekingTime, Value: "30"},
		}}
		if got := u.GetPreferredName(); got != "Ada L." {
			t.Errorf("GetPreferredName = %q", got)
		}
		if got := u.GetPreferredGreeting(); got != "Hi" {
			t.Errorf("GetPreferredGreeting = %q", got)
		}
		if got := u.GetPreferredView(); got != "Presentation" {
			t.Errorf("GetPreferredView = %q", got)
		}
		if got := u.GetSeekingTime(); got != 30 {
			t.Errorf("GetSeekingTime = %d, want 30", got)
		}
	})

	// Duplicate rows for one key are possible (nothing enforces uniqueness); the first
	// row in the slice wins, so it is first-write-wins, not last.
	t.Run("the first of two rows for a key wins", func(t *testing.T) {
		u := &User{Name: "Ada", Settings: []UserSetting{
			{Type: Greeting, Value: "first"},
			{Type: Greeting, Value: "second"},
		}}
		if got := u.GetPreferredGreeting(); got != "first" {
			t.Errorf("GetPreferredGreeting = %q, want %q", got, "first")
		}
	})
}

// A seeking time outside the offered choices would produce a player control that does
// not match its label, so anything unexpected collapses to the default.
func TestGetSeekingTime(t *testing.T) {
	tests := []struct {
		name string
		user *User
		want int
	}{
		{"an anonymous caller gets the default", nil, 10},
		{"5 seconds is accepted", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "5"}}}, 5},
		{"10 seconds is accepted", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "10"}}}, 10},
		{"30 seconds is accepted", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "30"}}}, 30},
		{"an unoffered value falls back", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "17"}}}, 10},
		{"a negative value falls back", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "-5"}}}, 10},
		{"a non numeric value falls back", &User{Settings: []UserSetting{{Type: SeekingTime, Value: "fast"}}}, 10},
		{"an empty value falls back", &User{Settings: []UserSetting{{Type: SeekingTime, Value: ""}}}, 10},
		// The column is JSON for other settings, so a JSON number is a plausible
		// mistake; strconv.Atoi rejects it and the default applies.
		{"a quoted number falls back", &User{Settings: []UserSetting{{Type: SeekingTime, Value: `"10"`}}}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.GetSeekingTime(); got != tt.want {
				t.Errorf("GetSeekingTime = %d, want %d", got, tt.want)
			}
		})
	}
}

// The playback speed rows are JSON written by the client; malformed content must not
// take the player down with it.
func TestGetPlaybackSpeeds(t *testing.T) {
	t.Run("an anonymous caller gets the defaults", func(t *testing.T) {
		var u *User
		if got := u.GetPlaybackSpeeds(); len(got) != len(defaultPlaybackSpeeds) {
			t.Errorf("GetPlaybackSpeeds returned %d entries, want %d", len(got), len(defaultPlaybackSpeeds))
		}
		if got := u.GetCustomSpeeds(); len(got) != 0 {
			t.Errorf("GetCustomSpeeds = %v, want empty", got)
		}
	})

	t.Run("a stored setting replaces the defaults", func(t *testing.T) {
		u := &User{Settings: []UserSetting{{
			Type:  CustomPlaybackSpeeds,
			Value: `[{"speed":1,"enabled":true},{"speed":2,"enabled":false}]`,
		}}}
		speeds := u.GetPlaybackSpeeds()
		if len(speeds) != 2 || speeds[0].Speed != 1 || speeds[1].Enabled {
			t.Errorf("GetPlaybackSpeeds = %+v", speeds)
		}
		if enabled := speeds.GetEnabled(); len(enabled) != 1 || enabled[0] != 1 {
			t.Errorf("GetEnabled = %v, want [1]", enabled)
		}
	})

	t.Run("malformed json falls back to the defaults", func(t *testing.T) {
		for _, value := range []string{"", "{", "null-ish", `{"speed":1}`} {
			u := &User{Settings: []UserSetting{{Type: CustomPlaybackSpeeds, Value: value}}}
			if got := u.GetPlaybackSpeeds(); len(got) != len(defaultPlaybackSpeeds) {
				t.Errorf("value %q: GetPlaybackSpeeds returned %d entries, want the %d defaults", value, len(got), len(defaultPlaybackSpeeds))
			}
		}
	})

	t.Run("malformed custom speeds fall back to empty", func(t *testing.T) {
		for _, value := range []string{"", "[1,", `{"a":1}`} {
			u := &User{Settings: []UserSetting{{Type: UserDefinedSpeeds, Value: value}}}
			if got := u.GetCustomSpeeds(); len(got) != 0 {
				t.Errorf("value %q: GetCustomSpeeds = %v, want empty", value, got)
			}
		}
	})
}

// The enabled list is what the player renders, so both halves (the toggled defaults and
// the user's own additions) must appear, in ascending order.
func TestGetEnabledPlaybackSpeeds(t *testing.T) {
	t.Run("the defaults are the enabled ones in order", func(t *testing.T) {
		u := &User{}
		want := []float32{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2}
		assertSpeeds(t, u.GetEnabledPlaybackSpeeds(), want)
	})

	t.Run("custom speeds are merged and sorted", func(t *testing.T) {
		u := &User{Settings: []UserSetting{
			{Type: CustomPlaybackSpeeds, Value: `[{"speed":1,"enabled":true},{"speed":2,"enabled":false}]`},
			{Type: UserDefinedSpeeds, Value: `[3, 0.1]`},
		}}
		assertSpeeds(t, u.GetEnabledPlaybackSpeeds(), []float32{0.1, 1, 3})
	})
}

func assertSpeeds(t *testing.T, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("speeds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("speeds = %v, want %v", got, want)
		}
	}
}

// Auto skip drives whether recorded lectures jump over silence; a broken row must be
// reported rather than quietly flipping the feature on.
func TestGetAutoSkipEnabled(t *testing.T) {
	tests := []struct {
		name        string
		settings    []UserSetting
		wantEnabled bool
		wantErr     bool
	}{
		{"no setting defaults to off", nil, false, false},
		{"an explicit true", []UserSetting{{Type: AutoSkip, Value: `{"enabled":true}`}}, true, false},
		{"an explicit false", []UserSetting{{Type: AutoSkip, Value: `{"enabled":false}`}}, false, false},
		// An object without the field decodes fine and leaves the zero value.
		{"an empty object is off", []UserSetting{{Type: AutoSkip, Value: `{}`}}, false, false},
		{"malformed json is an error and off", []UserSetting{{Type: AutoSkip, Value: `{"enabled":`}}, false, true},
		{"an empty value is an error and off", []UserSetting{{Type: AutoSkip, Value: ``}}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&User{Settings: tt.settings}).GetAutoSkipEnabled()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAutoSkipEnabled error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Enabled != tt.wantEnabled {
				t.Errorf("GetAutoSkipEnabled = %v, want %v", got.Enabled, tt.wantEnabled)
			}
		})
	}
}

// The cool down exists so a display name cannot be churned to impersonate someone; it
// is measured from the setting row's UpdatedAt.
func TestPreferredNameChangeAllowed(t *testing.T) {
	const threeMonths = time.Hour * 24 * 30 * 3

	tests := []struct {
		name     string
		settings []UserSetting
		want     bool
	}{
		{"no preferred name was ever set", nil, true},
		{
			"a name set just now is locked",
			[]UserSetting{{Model: gorm.Model{UpdatedAt: time.Now()}, Type: PreferredName, Value: "Ada"}},
			false,
		},
		{
			"a name set four months ago may be changed",
			[]UserSetting{{Model: gorm.Model{UpdatedAt: time.Now().Add(-threeMonths - time.Hour)}, Type: PreferredName, Value: "Ada"}},
			true,
		},
		{
			"a different setting changed recently does not lock the name",
			[]UserSetting{{Model: gorm.Model{UpdatedAt: time.Now()}, Type: Greeting, Value: "Hi"}},
			true,
		},
		// A row loaded without timestamps (or built in memory) reads as very old and
		// therefore unlocks the name; worth knowing before trusting this as a guard.
		{
			"a row with no timestamp is treated as long past",
			[]UserSetting{{Type: PreferredName, Value: "Ada"}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (&User{Settings: tt.settings}).PreferredNameChangeAllowed(); got != tt.want {
				t.Errorf("PreferredNameChangeAllowed = %v, want %v", got, tt.want)
			}
		})
	}
}

// courseIDs makes the semester assertions order independent; CoursesForSemester builds
// its result from a map, so its order is not defined.
func courseIDs(courses []Course) map[uint]bool {
	ids := make(map[uint]bool, len(courses))
	for _, c := range courses {
		ids[c.ID] = true
	}
	return ids
}

func assertCourseIDs(t *testing.T, got []Course, want ...uint) {
	t.Helper()
	ids := courseIDs(got)
	if len(ids) != len(got) {
		t.Errorf("result contains duplicate courses: %v", got)
	}
	if len(ids) != len(want) {
		t.Fatalf("got course ids %v, want %v", ids, want)
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("got course ids %v, want %v", ids, want)
		}
	}
}

func semesterCourse(id uint, year int, term string) Course {
	return Course{Model: gorm.Model{ID: id}, Year: year, TeachingTerm: term}
}

// The semester pickers feed the "my courses" page; a course landing in the wrong
// semester either hides a running course or resurrects an old one.
func TestCoursesForSemester(t *testing.T) {
	shared := semesterCourse(1, 2023, "W")
	u := &User{
		Model: gorm.Model{ID: 1},
		Role:  StudentType,
		Courses: []Course{
			shared,
			semesterCourse(2, 2023, "W"),
			semesterCourse(3, 2023, "S"),
			semesterCourse(4, 2024, "W"),
		},
		AdministeredCourses: []Course{
			shared, // also enrolled: must appear once, not twice
			semesterCourse(5, 2023, "W"),
		},
	}

	t.Run("enrolled and administered courses are merged without duplicates", func(t *testing.T) {
		assertCourseIDs(t, u.CoursesForSemester(2023, "W"), 1, 2, 5)
	})

	// The term is matched exactly, so the same year in the other term must not leak.
	t.Run("the other term of the same year is excluded", func(t *testing.T) {
		assertCourseIDs(t, u.CoursesForSemester(2023, "S"), 3)
	})

	t.Run("a semester with nothing returns nothing", func(t *testing.T) {
		assertCourseIDs(t, u.CoursesForSemester(2022, "S"))
	})
}

func TestAdministeredCoursesForSemesters(t *testing.T) {
	u := &User{
		Model: gorm.Model{ID: 1},
		Role:  LecturerType,
		AdministeredCourses: []Course{
			semesterCourse(1, 2023, "S"),
			semesterCourse(2, 2023, "W"),
			semesterCourse(3, 2024, "S"),
		},
	}

	t.Run("only the listed semesters are returned", func(t *testing.T) {
		got := u.AdministeredCoursesForSemesters([]Semester{{Year: 2023, TeachingTerm: "S"}, {Year: 2024, TeachingTerm: "S"}})
		assertCourseIDs(t, got, 1, 3)
	})

	// The list is matched element-wise, not as a range: a year alone selects nothing.
	t.Run("an empty semester list returns an empty slice", func(t *testing.T) {
		got := u.AdministeredCoursesForSemesters(nil)
		if got == nil {
			t.Error("got nil, want an empty slice so callers can range and marshal it")
		}
		assertCourseIDs(t, got)
	})
}

// The range endpoints are inclusive, and within a year S comes before W; both are easy
// to get backwards and would silently drop a semester from the course list.
func TestAdministeredCoursesBetweenSemesters(t *testing.T) {
	u := &User{
		Model: gorm.Model{ID: 1},
		Role:  LecturerType,
		AdministeredCourses: []Course{
			semesterCourse(1, 2022, "W"),
			semesterCourse(2, 2023, "S"),
			semesterCourse(3, 2023, "W"),
			semesterCourse(4, 2024, "S"),
		},
	}

	tests := []struct {
		name  string
		first Semester
		last  Semester
		want  []uint
	}{
		{
			"both endpoints are included",
			Semester{Year: 2022, TeachingTerm: "W"}, Semester{Year: 2024, TeachingTerm: "S"},
			[]uint{1, 2, 3, 4},
		},
		{
			"summer sorts before winter inside a year",
			Semester{Year: 2023, TeachingTerm: "W"}, Semester{Year: 2024, TeachingTerm: "S"},
			[]uint{3, 4},
		},
		{
			"a range ending in summer excludes that year's winter",
			Semester{Year: 2022, TeachingTerm: "W"}, Semester{Year: 2023, TeachingTerm: "S"},
			[]uint{1, 2},
		},
		{
			"a single semester range matches only that semester",
			Semester{Year: 2023, TeachingTerm: "S"}, Semester{Year: 2023, TeachingTerm: "S"},
			[]uint{2},
		},
		{
			"a range before everything is empty",
			Semester{Year: 2010, TeachingTerm: "S"}, Semester{Year: 2011, TeachingTerm: "W"},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCourseIDs(t, u.AdministeredCoursesBetweenSemesters(tt.first, tt.last), tt.want...)
		})
	}
}

// These two power the "courses I attend" list, which must not repeat courses already
// shown as "courses I teach".
func TestCoursesWithoutAdministeredCourses(t *testing.T) {
	both := semesterCourse(1, 2023, "W")
	u := &User{
		Model: gorm.Model{ID: 1},
		Role:  LecturerType,
		Courses: []Course{
			both,
			semesterCourse(2, 2023, "W"),
			semesterCourse(3, 2023, "S"),
		},
		AdministeredCourses: []Course{both},
	}

	t.Run("a course that is also administered is dropped", func(t *testing.T) {
		got := u.CoursesForSemestersWithoutAdministeredCourses([]Semester{{Year: 2023, TeachingTerm: "W"}})
		assertCourseIDs(t, got, 2)
	})

	t.Run("the same holds over a semester range", func(t *testing.T) {
		got := u.CoursesBetweenSemestersWithoutAdministeredCourses(
			Semester{Year: 2023, TeachingTerm: "S"}, Semester{Year: 2023, TeachingTerm: "W"})
		assertCourseIDs(t, got, 2, 3)
	})

	// The exclusion goes through IsAdminOfCourse, which is true for every course when
	// the caller is a server admin, so an admin's attended-course list comes back
	// empty. Surprising, but pinned as the current behaviour.
	t.Run("an admin sees none of their enrolled courses here", func(t *testing.T) {
		admin := &User{Model: gorm.Model{ID: 2}, Role: AdminType, Courses: u.Courses}
		assertCourseIDs(t, admin.CoursesForSemestersWithoutAdministeredCourses([]Semester{{Year: 2023, TeachingTerm: "W"}}))
	})

	t.Run("both return an empty slice rather than nil", func(t *testing.T) {
		empty := &User{Model: gorm.Model{ID: 3}}
		if empty.CoursesForSemestersWithoutAdministeredCourses(nil) == nil {
			t.Error("CoursesForSemestersWithoutAdministeredCourses returned nil")
		}
		if empty.CoursesBetweenSemestersWithoutAdministeredCourses(Semester{}, Semester{}) == nil {
			t.Error("CoursesBetweenSemestersWithoutAdministeredCourses returned nil")
		}
	})
}

// The magic year 1234 marks the demo course created for new lecturers; matching it too
// loosely would hide the onboarding prompt from everyone.
func TestHasTestCourse(t *testing.T) {
	tests := []struct {
		name string
		user *User
		want bool
	}{
		{"no administered courses", &User{}, false},
		{"an administered course in year 1234", &User{AdministeredCourses: []Course{semesterCourse(1, 1234, "W")}}, true},
		{"a real course only", &User{AdministeredCourses: []Course{semesterCourse(1, 2023, "W")}}, false},
		// Only the administered list counts: being enrolled in the demo course is not
		// the same as owning one.
		{"the test course is only enrolled, not administered", &User{Courses: []Course{semesterCourse(1, 1234, "W")}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.HasTestCourse(); got != tt.want {
				t.Errorf("HasTestCourse = %v, want %v", got, tt.want)
			}
		})
	}
}

// GetLoginString labels audit entries and the admin user list, where an empty label
// would make an action untraceable.
func TestGetLoginString(t *testing.T) {
	tests := []struct {
		name string
		user *User
		want string
	}{
		{"a nil user is the system", nil, "- System -"},
		{"an email wins over the lrz id", &User{Email: sql.NullString{String: "a@b.de", Valid: true}, LrzID: "ab12cde"}, "a@b.de"},
		{"the lrz id is used without an email", &User{LrzID: "ab12cde"}, "ab12cde"},
		// Valid is not consulted, only the string, so an invalid-but-populated
		// NullString is still used as the login.
		{"an invalid null string with content is still used", &User{Email: sql.NullString{String: "a@b.de"}, LrzID: "ab12cde"}, "a@b.de"},
		{"a user with neither has no label", &User{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.GetLoginString(); got != tt.want {
				t.Errorf("GetLoginString = %q, want %q", got, tt.want)
			}
		})
	}
}

// BeforeCreate is the only guard on the varchar(80) name column, and it also decides
// whether a whitespace-only name counts as empty.
func TestUserBeforeCreate(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantName string
		wantErr  error
	}{
		{"a normal name is kept", "Ada", "Ada", nil},
		{"surrounding whitespace is trimmed", "  Ada  ", "Ada", nil},
		{"a whitespace only name is empty", "   ", "", ErrUsernameNoText},
		{"an empty name is rejected", "", "", ErrUsernameNoText},
		{"a name at the limit is accepted", strings.Repeat("a", MaxUsernameLength), strings.Repeat("a", MaxUsernameLength), nil},
		// The limit is bytes, not runes, so a name of 80 multi-byte characters is
		// rejected even though the column counts characters.
		{"a name one over the limit is rejected", strings.Repeat("a", MaxUsernameLength+1), "", ErrUsernameTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{Name: tt.in}
			err := u.BeforeCreate(nil)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("BeforeCreate error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", u.Name, tt.wantName)
			}
		})
	}
}

// argon2.Version is baked into every stored hash; if the library ever bumps it, every
// existing password stops verifying, so the constant is pinned here as a tripwire.
func TestArgon2VersionIsPinned(t *testing.T) {
	if argon2.Version != 0x13 {
		t.Errorf("argon2.Version = %#x, want 0x13; existing stored hashes will no longer decode", argon2.Version)
	}
}
