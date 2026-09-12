package tools

import (
	"net/url"
	"strconv"
	"strings"
)

// CanonicalURL builds the absolute links that go into <link rel="canonical"> and the
// share links, from the canonicalURL of the deployment config.
type CanonicalURL struct {
	url string
}

func NewCanonicalURL(url string) CanonicalURL {
	return CanonicalURL{url: url}
}

func (c CanonicalURL) Root() string {
	return c.url
}

func (c CanonicalURL) Course(year int, term string, slug string) string {
	return c.join("course", strconv.Itoa(year), term, slug)
}

func (c CanonicalURL) Stream(slug string, id uint, version string) string {
	return c.join("w", slug, strconv.FormatUint(uint64(id), 10), version)
}

func (c CanonicalURL) Login() string {
	return c.join("login")
}

func (c CanonicalURL) Info(version string) string {
	return c.join(version)
}

// join appends the segments to the configured base URL. It uses url.JoinPath rather
// than path.Join because the latter runs path.Clean, which collapses the "//" of the
// scheme into "https:/" and resolves any ".." a segment carries. Every segment is
// percent-escaped so that a caller-supplied value - a course slug is user-editable -
// lands as exactly one path segment and cannot climb out of the path it is joined into.
// Empty segments are dropped, so an absent stream version leaves no trailing slash.
//
// These methods are called straight from templates and cannot report an error, so a
// base URL that does not parse degrades to a root-relative path instead.
func (c CanonicalURL) join(segments ...string) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		escaped = append(escaped, escapeSegment(segment))
	}
	if c.url != "" {
		if joined, err := url.JoinPath(c.url, escaped...); err == nil {
			return joined
		}
	}
	// No usable base: a root-relative path still resolves from any page, which a bare
	// relative path would not.
	return "/" + strings.Join(escaped, "/")
}

// escapeSegment turns one path segment into its percent-encoded form. url.PathEscape
// leaves "." and ".." alone and url.JoinPath still resolves those as traversal, so a
// segment made up of nothing but dots is encoded by hand.
func escapeSegment(segment string) string {
	escaped := url.PathEscape(segment)
	if strings.Trim(escaped, ".") == "" {
		escaped = strings.ReplaceAll(escaped, ".", "%2E")
	}
	return escaped
}
