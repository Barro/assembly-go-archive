package main

import (
	// "fmt"
	"html/template"
	"io"
	// "log"
	"net/http"
	"os"
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

type EntryThumbnail struct {
	TargetPage       string
	Title            string
	Author           string
	DefaultThumbnail ImageInfo
	Thumbnails       []TypedImage
}

type Section struct {
	Name        string
	Description string
	Ongoing     bool
	Ordered     bool
	Entries     []EntryThumbnail
}

type PageContext struct {
	Title   string
	RootUrl string
	Url     string
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

func render_thumbnail(wr io.Writer, thumbnail EntryThumbnail) {
	t := template.Must(template.New("thumbnail").Parse(`
<a class="thumbnail" href="{{.TargetPage}}">
  <img class="thumbnail-image"
    src="{{.TargetPage}}/{{.DefaultThumbnail.Filename}}"
    alt="{{.Title}}"
  />
  {{.Title}}
  <span class="by">{{.Author}}</span>
</a>
`))
	t.Execute(wr, thumbnail)
}

func render_page(w http.ResponseWriter, r *http.Request) {
	render(w)
}

func render(w io.Writer) {
	context := PageContext{
		Title:   "",
		RootUrl: "http://localhost:4000",
		Url:     "http://localhost:4000",
	}
	render_header(w, context)

	thumbnail := EntryThumbnail{
		TargetPage: "/section/otsikko-by-autori",
		Title:      "otsikko",
		Author:     "autori",
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

func main() {
	render(os.Stdout)
	// http.HandleFunc("/", render_page)
	// log.Fatal(http.ListenAndServe(":4000", nil))
}
