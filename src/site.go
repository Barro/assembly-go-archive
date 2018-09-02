package site

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
)

type Resolution struct {
	X int
	Y int
}

type ImageInfo struct {
	Filename   string
	Resolution Resolution
}

type TypedImage struct {
	Type_ string
	Image ImageInfo
}

type ThumbnailedEntry struct {
	Path             string
	Key              string
	Title            string
	Author           string
	DefaultThumbnail ImageInfo
	Thumbnails       []TypedImage
}

type Section struct {
	Path        string
	Key         string
	Name        string
	Description string
	IsRanked    bool
	Entries     []ThumbnailedEntry
}

type Year struct {
	Path     string
	Key      string
	Name     string
	Sections []Section
}

type PageContext struct {
	Title   string
	RootUrl string
	Url     string
}

func in_array(array []ThumbnailedEntry, entry ThumbnailedEntry) bool {
	for _, array_entry := range array {
		if array_entry.Path == entry.Path {
			return true
		}
	}
	return false
}

// Randomly selects a number of entries by taking no more than 2 from
// each section.
func random_select_entries(year Year, amount int) []ThumbnailedEntry {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * 2)
	var result []ThumbnailedEntry
	for _, index_value := range section_indexes {
		if len(result) == amount {
			break
		}
		index := index_value % total_sections
		entries := year.Sections[index].Entries
		entry := entries[rand.Intn(len(entries))]
		if in_array(result, entry) {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// Takes a preview sample of entries in a section.
//
// If the section is ranked, returns the top "amount" entries. If it's
// not, returns random selection of entries. This is to promote the
// best ranked entries where it's possible.
func peek_section_entries(section Section, amount int) []ThumbnailedEntry {
	if section.IsRanked {
		return section.Entries[:amount]
	}

	var result []ThumbnailedEntry
	for _, index := range rand.Perm(len(section.Entries))[:amount] {
		result = append(result, section.Entries[index])
	}
	return result
}

/*
{
    "title": "Title",
    "author": "Author",
    "asset": "<embed>raw-asset-html</embed>",
    "thumbnail": {"path": "filename.jpeg", "size": "160x90", "type": "image/jpeg"},
    "thumbnails": [{"path": "filename.png", "size": "160x90", "type": "image/png"}],
    "description": "<p>raw-html</p>",
    "external": [
        {"Download": ["<p>raw-html</p>"]},
        {"View on": ["<p>raw-html</p>"]},
    ],
}
*/
type ThumbnailInfo struct {
	Path     string
	Checksum *string
	Size     Resolution
	Type     string
}

type TypedThumbnails struct {
	Type       string
	Thumbnails []ThumbnailInfo
}

type Thumbnails struct {
	Default ThumbnailInfo
	Extra   []TypedThumbnails
}

type ExternalLinksSection struct {
	Name  string
	Links []string
}

// Structure that has all known data about an entry.
type EntryInfo struct {
	Path          string
	Key           string
	Title         string
	Author        string
	Asset         string
	Description   string
	ExternalLinks []ExternalLinksSection
	Thumbnails    Thumbnails
}

func string_to_resolution(value string) Resolution {
	return Resolution{0, 0}
}

// Creates a checksum of a file that is appropriate for caching for
// long time periods. For less than 1 year, though.
func create_file_checksum(filename string) (string, error) {
	stats, err := os.Stat(filename)
	if err != nil {
		return "", err
	}
	modified := stats.ModTime()
	// Same values can be encountered every 136 years.
	value := uint32(modified.Unix())

	buffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(buffer, value)
	str := base64.RawURLEncoding.EncodeToString(buffer)
	// All 32 bit values fit in 6 characters (= 36 bits space).
	return str[:6], nil
}

func read_entry_info(directory string, url_path string) (EntryInfo, error) {
	data, err := ioutil.ReadFile(filepath.Join(directory, "meta.json"))
	key := filepath.Base(directory)
	entry := EntryInfo{Path: url_path, Key: key}
	var meta_json_raw interface{}
	err_unmarshal := json.Unmarshal(data, &meta_json_raw)
	if err_unmarshal != nil {
		return entry, err_unmarshal
	}

	meta_root := meta_json_raw.(map[string]interface{})
	entry.Title = meta_root["title"].(string)
	entry.Author = meta_root["author"].(string)
	entry.Asset = meta_root["asset"].(string)

	_json_to_thumbnail := func(value map[string]string) (ThumbnailInfo, error) {
		checksum, err := create_file_checksum(filepath.Join(directory, value["path"]))
		if err != nil {
			return ThumbnailInfo{}, err
		}
		return ThumbnailInfo{
			url_path + "/" + value["path"],
			&checksum,
			string_to_resolution(value["resolution"]),
			value["type"]}, nil
	}
	entry.Thumbnails.Default, err = _json_to_thumbnail(
		meta_root["thumbnail"].(map[string]string))
	if err != nil {
		return entry, err
	}

	return entry, nil
}

func entry_info_to_thumbnail(entry EntryInfo) ThumbnailedEntry {
	return ThumbnailedEntry{}
}

func render_header(wr io.Writer, context PageContext) {
	t := template.Must(template.New("header").Parse(`
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">

<title>
{{ if .Title }}
  {{ .Title }} &ndash; Assembly Archive
{{ else }}
  Assembly Archive
{{ end }}
</title>

<tal:block tal:content="structure provider:metadata" />

<meta property="fb:page_id" content="183599045012296" />

<link rel="shortcut icon" type="image/vnd.microsoft.icon"
      href="/static/images/favicon.ico" />
<link rel="icon" type="image/vnd.microsoft.icon"
      href="/static/images/favicon.ico" />

<!-- List of CSS files that are optimized and appended into one with yui-compressor -->
<!-- <link rel="stylesheet" href="/static/css/reset.css" /> -->
<!-- <link rel="stylesheet" href="/static/css/960.css" /> -->
<!-- <link rel="stylesheet" href="/static/css/text.css" /> -->
<!-- <link rel="stylesheet" href="/static/css/style.css" /> -->

<link rel="stylesheet" href="/static/allstyles-min.css" />

<meta name="viewport" content="width=640" />

<link rel="search" type="application/opensearchdescription+xml"
title="Assembly Archive" href="{{ .RootUrl}}/@@osdd.xml" />

</head>
<body>
`))
	t.Execute(wr, context)
}

func render_thumbnail(wr io.Writer, thumbnail ThumbnailedEntry) {
	t := template.Must(template.New("thumbnail").Parse(`
<a class="thumbnail" href="{{.Path}}">
  <img class="thumbnail-image"
    src="{{.Path}}/{{.DefaultThumbnail.Filename}}"
    alt="{{.Title}}"
  />
  {{.Title}}
  <span class="by">{{.Author}}</span>
</a>
`))
	t.Execute(wr, thumbnail)
}

type SiteSettings struct {
	DataDir   string
	StaticDir string
}

func render_request(settings SiteSettings, w http.ResponseWriter, r *http.Request) {
	render(w)
}

func SiteRenderer(settings SiteSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render_request(settings, w, r)
	}
}

func render(w io.Writer) {
	context := PageContext{
		Title:   "",
		RootUrl: "http://localhost:4000",
		Url:     "http://localhost:4000",
	}
	render_header(w, context)

	thumbnail := ThumbnailedEntry{
		Path:   "/section/otsikko-by-autori",
		Key:    "otsikko-by-autori",
		Title:  "otsikko",
		Author: "autori",
		DefaultThumbnail: ImageInfo{
			Filename: "thumbnail.png",
			Resolution: Resolution{
				X: 10,
				Y: 10,
			},
		},
		Thumbnails: []TypedImage{},
	}

	// }
	render_thumbnail(w, thumbnail)
}
