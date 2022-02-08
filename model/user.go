package model

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
	"strings"
)

const (
	AdminType    = 1
	LecturerType = 2
	GenericType  = 3
	StudentType  = 4
)

type User struct {
	gorm.Model

	Name                string         `gorm:"not null"`
	Email               sql.NullString `gorm:"unique;default:null"`
	MatriculationNumber string         `gorm:"unique;default:null"`
	LrzID               string
	Role                uint     `gorm:"default:4"` // AdminType = 1, LecturerType = 2, GenericType = 3, StudentType  = 4
	Password            string   `gorm:"default:null"`
	Courses             []Course `gorm:"many2many:course_users"` // courses a lecturer invited this user to
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
	return u.Role == AdminType || u.ID == course.UserID
}

func (u *User) CoursesForSemester(year int, term string, context context.Context) []Course {
	span := sentry.StartSpan(context, "User.CoursesForSemester")
	defer span.Finish()
	var cRes []Course
	for _, c := range u.Courses {
		if c.Year == year && c.TeachingTerm == term {
			cRes = append(cRes, c)
		}
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
