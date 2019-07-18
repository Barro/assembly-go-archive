package site

import (
	"base"
	"bufio"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"server"
	"state"
	"text/template"
)

type PageContext struct {
	Title       string
	RootUrl     string
	Url         string
	CurrentYear int
	SiteState   *state.SiteState
	Navigation  *state.Navigable
}

type SectionThumbnails struct {
	Settings *base.SiteSettings
	Entries  []*base.EntryInfo
}

func in_array(array []*base.ThumbnailedEntry, entry *base.ThumbnailedEntry) bool {
	for _, array_entry := range array {
		if array_entry.Path == entry.Path {
			return true
		}
	}
	return false
}

// Randomly selects a number of entries by taking no more than 2 from
// each section.
func random_select_entries(year base.Year, amount int) []*base.ThumbnailedEntry {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * 2)
	var result []*base.ThumbnailedEntry
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
func peek_section_entries(section base.Section, amount int) []*base.ThumbnailedEntry {
	if section.IsRanked {
		return section.Entries[:amount]
	}

	var result []*base.ThumbnailedEntry
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

func render_page(ctx PageContext, wr io.Writer) {
	t := template.Must(template.ParseFiles("templates/layout.html.tmpl"))
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

func view_author_title(entry base.EntryInfo) string {
	var author_title string
	if entry.Author == "" {
		author_title = entry.Title
	} else {
		author_title = entry.Title + " by " + entry.Author
	}
	return html.EscapeString(author_title)
}

type GalleryRenderer struct {
	Settings *base.SiteSettings
	Template *template.Template
}

func NewGalleryRenderer(settings *base.SiteSettings) (*GalleryRenderer, error) {
	ti := template.New("templates/gallery.html.tmpl")
	functions := template.FuncMap{}
	functions["view_author_title"] = view_author_title
	ti = ti.Funcs(functions)
	template_data, data_err := ioutil.ReadFile("templates/gallery.html.tmpl")
	if data_err != nil {
		return nil, data_err
	}
	t, template_err := ti.Parse(string(template_data))
	if template_err != nil {
		return nil, template_err
	}
	renderer := GalleryRenderer{
		Settings: settings,
		Template: t,
	}
	return &renderer, nil
}

func (r *GalleryRenderer) Render(w http.ResponseWriter, entries []*base.EntryInfo) error {
	ctx := SectionThumbnails{
		Settings: r.Settings,
		Entries:  entries,
	}
	wr := bufio.NewWriterSize(w, 1024*64)
	err := r.Template.Execute(wr, ctx)
	if err != nil {
		server.Ise(w)
		return err
	} else {
		return wr.Flush()
	}
	return nil
}

func render_gallery(w http.ResponseWriter, settings base.SiteSettings) {
	renderer, err_renderer := NewGalleryRenderer(&settings)
	if err_renderer != nil {
		server.Ise(w)
		log.Print(err_renderer)
		return
	}
	entry1 := base.EntryInfo{
		Path:          "path",
		Key:           "key",
		Title:         "title",
		Author:        "author",
		Asset:         "asset",
		Description:   "description",
		ExternalLinks: []base.ExternalLinksSection{},
		Thumbnails: base.Thumbnails{
			Default: base.ThumbnailInfo{
				Path:     "/absolute/path",
				Checksum: nil,
				Size: base.Resolution{
					X: 160,
					Y: 90,
				},
				Type: "image/png",
			},
			Extra: []base.TypedThumbnails{},
		},
	}
	entries := make([]*base.EntryInfo, 5)
	for i := 0; i < len(entries); i++ {
		entries[i] = &entry1
	}
	for i := 0; i < 25; i++ {
		if err := renderer.Render(w, entries); err != nil {
			log.Print(err)
			break
		}
	}
}

type RequestHandlerFunc func(
	settings base.SiteSettings,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request)

func handle_entry(
	settings base.SiteSettings,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("%v %s\n", path_elements, r.URL)
	render(w, settings)
}

func handle_section(
	settings base.SiteSettings,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("%v %s\n", path_elements, r.URL)
	render(w, settings)
}

func handle_year(
	settings base.SiteSettings,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	// fmt.Printf("year %v %s\n", path_elements, r.URL)
	render(w, settings)
}

func handle_main(
	settings base.SiteSettings,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	// fmt.Printf("main %v %s\n", path_elements, r.URL)
	render(w, settings)
}

type RequestHandler struct {
	regex    *regexp.Regexp
	callback RequestHandlerFunc
}

var HANDLERS = []RequestHandler{
	{regexp.MustCompile(`^(?P<Year>\d{4})/(?P<Section>[a-z0-9\-]+)/(?P<Entry>[a-z0-9\-]+)/?$`),
		handle_entry},
	{regexp.MustCompile(`^(?P<Year>\d{4})/(?P<Section>[a-z0-9\-]+)/?$`), handle_section},
	{regexp.MustCompile(`^(?P<Year>\d{4})/?$`), handle_year},
	{regexp.MustCompile("^$"), handle_main},
}

func route_request(settings base.SiteSettings,
	w http.ResponseWriter,
	r *http.Request) {
	path := r.URL.EscapedPath()
	found := false
	for _, handler := range HANDLERS {
		path_regex := handler.regex
		match := path_regex.FindStringSubmatch(path)
		if match != nil {
			path_elements := make(map[string]string)
			for i, name := range handler.regex.SubexpNames() {
				path_elements[name] = match[i]
			}
			handler.callback(settings, path_elements, w, r)
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("NOTFOUND %s\n", r.URL)
		http.NotFound(w, r)
	}
}

func SiteRenderer(settings base.SiteSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route_request(settings, w, r)
	}
}

func render(w http.ResponseWriter, settings base.SiteSettings) {
	/*
		ctx := PageContext{
			Title:   "",
			RootUrl: "http://localhost:4000",
			Url:     "http://localhost:4000",
		}
		render_page(ctx, w)

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
	*/
	render_gallery(w, settings)
}
