package model

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
	"strings"
	"time"
)

const (
	AdminType    = 1
	LecturerType = 2
	GenericType  = 3
	StudentType  = 4
)

type User struct {
	gorm.Model

	Name                string         `gorm:"not null" json:"name"`
	LastName            *string        `json:"-"`
	Email               sql.NullString `gorm:"type:varchar(256); uniqueIndex; default:null" json:"-"`
	MatriculationNumber string         `gorm:"type:varchar(256); uniqueIndex; default:null" json:"-"`
	LrzID               string         `json:"-"`
	Role                uint           `gorm:"default:4" json:"-"` // AdminType = 1, LecturerType = 2, GenericType = 3, StudentType  = 4
	Password            string         `gorm:"default:null" json:"-"`
	Courses             []Course       `gorm:"many2many:course_users" json:"-"` // courses a lecturer invited this user to
	AdministeredCourses []Course       `gorm:"many2many:course_admins"`         // courses this user is an admin of

	Settings []UserSetting `gorm:"foreignkey:UserID"`
}

type UserSettingType int

const (
	PreferredName UserSettingType = iota + 1
	Greeting
	EnableChromecast
	CustomPlaybackSpeeds
)

type UserSetting struct {
	gorm.Model

	UserID uint            `gorm:"not null"`
	Type   UserSettingType `gorm:"not null"`
	Value  string          `gorm:"not null"` //json encoded setting
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

// GetPreferredGreeting returns the preferred greeting of the user if set, otherwise Moin
func (u User) GetPreferredGreeting() string {
	for _, setting := range u.Settings {
		if setting.Type == Greeting {
			return setting.Value
		}
	}
	return "Moin"
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

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// IsAdminOfCourse checks if the user is an admin of the course
func (u *User) IsAdminOfCourse(course Course) bool {
	for _, c := range u.AdministeredCourses {
		if c.ID == course.ID {
			return true
		}
	}
	return u.Role == AdminType || course.UserID == u.ID
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
	var cMap = make(map[uint]Course)
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
	ErrInvalidHash                     = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion             = errors.New("incompatible version of argon2")
	p                      argonParams = argonParams{
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
	if u.Email.String != "" {
		return u.Email.String
	}
	return u.LrzID
}
