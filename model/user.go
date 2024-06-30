package model

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

const (
	AdminType         = 1
	MaintainerType    = 2
	LecturerType      = 3
	GenericType       = 4
	StudentType       = 5
	maxUsernameLength = 80
)

var (
	ErrUsernameTooLong = errors.New("username is too long")
	ErrUsernameNoText  = errors.New("username has no text")
)

type User struct {
	gorm.Model

	Name                string         `gorm:"type:varchar(80); not null" json:"name"`
	LastName            *string        `json:"-"`
	Email               sql.NullString `gorm:"type:varchar(256); uniqueIndex; default:null" json:"-"`
	MatriculationNumber string         `gorm:"type:varchar(256); uniqueIndex; default:null" json:"-"`
	LrzID               string         `json:"-"`
	Role                uint           `gorm:"default:5" json:"-"` // AdminType = 1, MaintainerType = 2, LecturerType = 3, GenericType = 4, StudentType  = 5
	Password            string         `gorm:"default:null" json:"-"`
	Courses             []Course       `gorm:"many2many:course_users" json:"-"` // courses a lecturer invited this user to
	AdministeredCourses []Course       `gorm:"many2many:course_admins"`         // courses this user is an admin of
	PinnedCourses       []Course       `gorm:"many2many:pinned_courses"`
	AdministeredSchools []School       `gorm:"many2many:school_admins"`

	Settings  []UserSetting `gorm:"foreignkey:UserID"`
	Bookmarks []Bookmark    `gorm:"foreignkey:UserID" json:"-"`
}

type UserSettingType int

const (
	PreferredName UserSettingType = iota + 1
	Greeting
	CustomPlaybackSpeeds
	SeekingTime
	UserDefinedSpeeds
	AutoSkip
	DefaultMode
)

type UserSetting struct {
	gorm.Model

	UserID uint            `gorm:"not null"`
	Type   UserSettingType `gorm:"not null"`
	Value  string          `gorm:"not null"` // json encoded setting
}

// GetPreferredName returns the preferred name of the user if set, otherwise the firstName from TUMOnline
func (u User) GetPreferredName() string {
	for _, setting := range u.Settings {
		if setting.Type == PreferredName {
			return setting.Value
		}
	}
	return u.Name
}

type PlaybackSpeedSetting struct {
	Speed   float32 `json:"speed"`
	Enabled bool    `json:"enabled"`
}

type CustomSpeeds []float32

type PlaybackSpeedSettings []PlaybackSpeedSetting

func (s PlaybackSpeedSettings) GetEnabled() (res []float32) {
	for _, setting := range s {
		if setting.Enabled {
			res = append(res, setting.Speed)
		}
	}
	return res
}

func (u *User) GetEnabledPlaybackSpeeds() (res []float32) {
	if u == nil {
		return []float32{1}
	}
	// Possibly, this could be collapsed into a single line, but readibility suffers.
	res = append(res, u.GetPlaybackSpeeds().GetEnabled()...)
	res = append(res, u.GetCustomSpeeds()...)
	sort.SliceStable(res, func(i, j int) bool {
		return res[i] < res[j]
	})
	return res
}

var defaultPlaybackSpeeds = PlaybackSpeedSettings{
	{0.25, false},
	{0.5, true},
	{0.75, true},
	{1, true},
	{1.25, true},
	{1.5, true},
	{1.75, true},
	{2, true},
	{2.5, false},
	{3, false},
	{3.5, false},
}

func (u *User) GetPlaybackSpeeds() (speeds PlaybackSpeedSettings) {
	if u == nil {
		return defaultPlaybackSpeeds
	}
	for _, setting := range u.Settings {
		if setting.Type == CustomPlaybackSpeeds {
			err := json.Unmarshal([]byte(setting.Value), &speeds)
			if err != nil {
				break
			}
			return speeds
		}
	}
	return defaultPlaybackSpeeds
}

func (u *User) GetCustomSpeeds() (speeds CustomSpeeds) {
	if u == nil {
		return []float32{}
	}
	for _, setting := range u.Settings {
		if setting.Type == UserDefinedSpeeds {
			err := json.Unmarshal([]byte(setting.Value), &speeds)
			if err != nil {
				break
			}
			return speeds
		}
	}
	return []float32{}
}

// GetPreferredGreeting returns the preferred greeting of the user if set, otherwise Moin
func (u User) GetPreferredGreeting() string {
	for _, setting := range u.Settings {
		if setting.Type == Greeting {
			return setting.Value
		}
	}
	return "Moin"
}

// GetSeekingTime returns the seeking time preference for the user.
// If the user is nil, the default seeking time of 15 seconds is returned.
func (u *User) GetSeekingTime() int {
	// Check if the user is nil
	if u == nil {
		return 15
	}
	// Check if the setting type is SeekingTime
	for _, setting := range u.Settings {
		if setting.Type == SeekingTime {
			// Attempt to convert the setting value from string to an integer
			seekingTime, err := strconv.Atoi(setting.Value)
			if err != nil {
				break
			}
			return seekingTime
		}
	}
	// If no seeking time setting is found, return the default seeking time
	return 15
}

// PreferredNameChangeAllowed returns false if the user has set a preferred name within the last 3 months, otherwise true
func (u User) PreferredNameChangeAllowed() bool {
	for _, setting := range u.Settings {
		if setting.Type == PreferredName && time.Since(setting.UpdatedAt) < time.Hour*24*30*3 {
			return false
		}
	}
	return true
}

// AutoSkipSetting wraps whether auto skip is enabled in JSON
type AutoSkipSetting struct {
	Enabled bool `json:"enabled"`
}

// GetAutoSkipEnabled returns whether the user has enabled auto skip
func (u User) GetAutoSkipEnabled() (AutoSkipSetting, error) {
	for _, setting := range u.Settings {
		if setting.Type == AutoSkip {
			var a AutoSkipSetting
			err := json.Unmarshal([]byte(setting.Value), &a)
			if err != nil {
				return AutoSkipSetting{Enabled: false}, err
			}
			return a, nil
		}
	}
	return AutoSkipSetting{Enabled: false}, nil
}

// DefaultModeSetting wraps whether the default stream mode for the user is beta
type DefaultModeSetting struct {
	Beta bool `json:"beta"`
}

func (u *User) GetDefaultMode() (DefaultModeSetting, error) {
	if u == nil {
		return DefaultModeSetting{Beta: false}, nil
	}
	for _, setting := range u.Settings {
		if setting.Type == DefaultMode {
			var m DefaultModeSetting
			err := json.Unmarshal([]byte(setting.Value), &m)
			if err != nil {
				return DefaultModeSetting{Beta: false}, err
			}
			return m, nil
		}
	}
	return DefaultModeSetting{Beta: false}, nil
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// IsAdminOfCourse checks if the user is an admin of the course
func (u *User) IsAdminOfCourse(course Course) bool {
	if u == nil {
		return false
	}
	for _, c := range u.AdministeredCourses {
		if c.ID == course.ID {
			return true
		}
	}

	return u.Role == AdminType || course.UserID == u.ID || u.IsMaintainerOfCourse(course)
}

func (u *User) IsEligibleToWatchCourse(course Course) bool {
	if course.Visibility == "loggedin" || course.Visibility == "public" {
		return true
	}
	for _, invCourse := range u.Courses {
		if invCourse.ID == course.ID {
			return true
		}
	}
	return u.IsAdminOfCourse(course)
}

func (u *User) CoursesForSemester(year int, term string, context context.Context) []Course {
	cMap := make(map[uint]Course)
	for _, c := range u.Courses {
		if c.Year == year && c.TeachingTerm == term {
			cMap[c.ID] = c
		}
	}
	for _, c := range u.AdministeredCourses {
		if c.Year == year && c.TeachingTerm == term {
			cMap[c.ID] = c
		}
	}
	var cRes []Course
	for _, c := range cMap {
		cRes = append(cRes, c)
	}
	return cRes
}

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
	p                      = argonParams{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}
)

func (u *User) SetPassword(password string) (err error) {
	if len(password) < 8 {
		return errors.New("password length insufficient")
	}
	encodedHash, err := GenerateFromPassword(password)
	if err != nil {
		return err
	}
	u.Password = encodedHash
	return nil
}

func (u *User) ComparePasswordAndHash(password string) (match bool, err error) {
	if u.Password == "" {
		return false, nil
	}
	// Extract the parameters, salt and derived key from the encoded password
	// hash.
	salt, hash, err := decodeHash(u.Password)
	if err != nil {
		return false, err
	}

	// Derive the key from the other password using the same parameters.
	otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Check that the contents of the hashed passwords are identical. Note
	// that we are using the subtle.ConstantTimeCompare() function for this
	// to help prevent timing attacks.
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeHash(encodedHash string) (salt, hash []byte, err error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, ErrIncompatibleVersion
	}

	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, err
	}

	salt, err = base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return nil, nil, err
	}
	p.saltLength = uint32(len(salt))

	hash, err = base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return nil, nil, err
	}
	p.keyLength = uint32(len(hash))

	return salt, hash, nil
}

func GenerateFromPassword(password string) (encodedHash string, err error) {
	// Generate a cryptographically secure random salt.
	salt, err := generateRandomBytes(p.saltLength)
	if err != nil {
		return "", err
	}

	// Pass the plaintext password, salt and parameters to the argon2.IDKey
	// function. This will generate a hash of the password using the Argon2id
	// variant.
	hash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Return a string using the standard encoded hash representation.
	encodedHash = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.memory, p.iterations, p.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

func generateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GetLoginString returns the email if it is set, otherwise the lrzID
func (u *User) GetLoginString() string {
	if u == nil {
		return "- System -"
	}
	if u.Email.String != "" {
		return u.Email.String
	}
	return u.LrzID
}

// BeforeCreate is a GORM hook that is called before a new user is created.
// Users won't be saved if any of these apply:
// - username is empty (after trimming)
// - username is too long (>maxUsernameLength)
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.Name = strings.TrimSpace(u.Name)
	if len(u.Name) > maxUsernameLength {
		return ErrUsernameTooLong
	}
	if len(u.Name) == 0 {
		return ErrUsernameNoText
	}
	return nil
}

// IsMaintainerOfCourse checks if the user is a maintainer of the course's school
func (u *User) IsMaintainerOfCourse(course Course) bool {
	if u == nil {
		return false
	}
	logger.Error("Checking if user is maintainer of course", "user", u.ID, "course", course.ID)
	logger.Error("User role and number of administered schools", "role", u.Role, "numSchools", len(u.AdministeredSchools))
	if u.Role == MaintainerType || u.Role == AdminType {
		for _, s := range u.AdministeredSchools {
			logger.Error("Checking if user is maintainer of course", "school", s.ID, "course", course.ID)
			if s.ID == course.SchoolID {
				return true
			}
		}
	}
	logger.Error("User is not maintainer of course", "user", u.ID, "course", course.ID)
	return false
}

// IsAdminOfSchool checks if the user is an admin of the school
func (u *User) IsAdminOfSchool(schoolId uint) bool {
	if u == nil {
		return false
	}
	for _, s := range u.AdministeredSchools {
		if s.ID == schoolId {
			return true
		}
	}
	return u.Role == AdminType
}
