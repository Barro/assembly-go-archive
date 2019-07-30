package base

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
)

type SiteSettings struct {
	SiteRoot     string
	DataDir      string
	StaticDir    string
	TemplatesDir string
}

type Resolution struct {
	X int
	Y int
}

type ImageInfo struct {
	Path     string
	Checksum string
	Size     Resolution
	Type     string
}

type Thumbnails struct {
	Default ImageInfo
}

type ExternalLink struct {
	Href     string
	Contents string
	Notes    string
}

type ExternalLinksSection struct {
	Name  string
	Links []ExternalLink
}

type Asset struct {
	Type string
	Data interface{}
}

// Structure that has all known data about an entry.
type Entry struct {
	Path          string
	Key           string
	Title         string
	Author        string
	Asset         Asset
	Description   string
	ExternalLinks []ExternalLinksSection
	Thumbnails    Thumbnails
}

type Section struct {
	Path        string
	Key         string
	Name        string
	Description string
	IsRanked    bool
	Entries     []*Entry
}

type Year struct {
	Year     int
	Path     string
	Key      string
	Name     string
	Sections []*Section
}

// Creates a checksum of a file that is appropriate for caching.
func CreateFileChecksum(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	result_full := hasher.Sum(nil)
	str := base64.RawURLEncoding.EncodeToString(result_full[:4])
	// All 32 bit values fit in 6 characters (= 36 bits space).
	return str[:6], nil
}
