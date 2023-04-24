package tools

import (
	"path"
	"strconv"
)

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
	return path.Join(c.url, "course", strconv.Itoa(year), term, slug)
}

func (c CanonicalURL) Stream(slug string, id uint, version string) string {
	return path.Join(c.url, "w", slug, strconv.Itoa(int(id)), version)
}
