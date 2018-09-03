package site

import (
	"base"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"state"
)

type PageContext struct {
	Title       string
	RootUrl     string
	Url         string
	CurrentYear int
	SiteState   *state.SiteState
}

func in_array(array []base.ThumbnailedEntry, entry base.ThumbnailedEntry) bool {
	for _, array_entry := range array {
		if array_entry.Path == entry.Path {
			return true
		}
	}
	return false
}

// Randomly selects a number of entries by taking no more than 2 from
// each section.
func random_select_entries(year base.Year, amount int) []base.ThumbnailedEntry {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * 2)
	var result []base.ThumbnailedEntry
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
func peek_section_entries(section base.Section, amount int) []base.ThumbnailedEntry {
	if section.IsRanked {
		return section.Entries[:amount]
	}

	var result []base.ThumbnailedEntry
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

func entry_info_to_thumbnail(entry base.EntryInfo) base.ThumbnailedEntry {
	return base.ThumbnailedEntry{}
}

func render_year(ctx PageContext, wr io.Writer, year *base.Year) {

}

func render_section(ctx PageContext, wr io.Writer, section *base.Section) {

}

func render_entry(ctx PageContext, wr io.Writer, entry *base.EntryInfo) {

}

func render_header(ctx PageContext, wr io.Writer) {
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
	t.Execute(wr, ctx)
}

func render_thumbnail(wr io.Writer, thumbnail base.ThumbnailedEntry) {
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

func render_request(
	settings base.SiteSettings,
	w http.ResponseWriter,
	r *http.Request) {
	render(w)
}

func SiteRenderer(settings base.SiteSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render_request(settings, w, r)
	}
}

func render(w io.Writer) {
	ctx := PageContext{
		Title:   "",
		RootUrl: "http://localhost:4000",
		Url:     "http://localhost:4000",
	}
	render_header(ctx, w)

	checksum := "sadf12"
	thumbnail := base.ThumbnailedEntry{
		Path:   "/section/otsikko-by-autori",
		Key:    "otsikko-by-autori",
		Title:  "otsikko",
		Author: "autori",
		Thumbnails: base.Thumbnails{
			Default: base.ThumbnailInfo{
				Path:     "/section/otsikko-by-autori/thumbnail.png",
				Checksum: &checksum,
				Size:     base.Resolution{10, 10},
				Type:     "image/png",
			},
			Extra: []base.TypedThumbnails{},
		},
	}

	// }
	render_thumbnail(w, thumbnail)
}
