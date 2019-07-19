package site

import (
	"base"
	"bufio"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"regexp"
	"server"
	"state"
	"strconv"
	"strings"
	"text/template"
)

var DEFAULT_MAIN_YEARS = 15

type SiteTemplates struct {
	Main        *template.Template
	Year        *template.Template
	Section     *template.Template
	Entry       *template.Template
	Description *template.Template
}

type Site struct {
	Settings  base.SiteSettings
	State     *state.SiteState
	Templates *SiteTemplates
}

type PageContext struct {
	Title       string
	RootUrl     string
	Url         string
	CurrentYear int
	Description string
	SiteState   *state.SiteState
	Navigation  *state.Navigable
}

type GalleryThumbnails struct {
	Path    string
	Title   string
	Entries []*base.EntryInfo
}

type InternalLink struct {
	Path     string
	Contents string
}

type MainContext struct {
	Galleries   []GalleryThumbnails
	YearsBefore InternalLink
	YearsAfter  InternalLink
	Context     PageContext
}

func in_array(array []*base.EntryInfo, entry *base.EntryInfo) bool {
	for _, array_entry := range array {
		if array_entry.Path == entry.Path {
			return true
		}
	}
	return false
}

func _bad_request(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("Bad request!\n"))
}

// Randomly selects a number of entries by taking no more than 2 from
// each section.
func random_select_entries(year *base.Year, amount int) []*base.EntryInfo {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * 2)
	var result []*base.EntryInfo
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
func peek_section_entries(section base.Section, amount int) []*base.EntryInfo {
	if section.IsRanked {
		return section.Entries[:amount]
	}

	var result []*base.EntryInfo
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

func LoadTemplates(settings *base.SiteSettings) (SiteTemplates, error) {
	var templates SiteTemplates
	data := "asdf"
	var generic *template.Template
	{
		generic = template.New("thumbnails")
	}
	{
		t := template.Must(generic.Clone()).New("main")
		templates.Main = template.Must(t.Parse(data))
	}
	{
		t := template.New("year")
		templates.Year = template.Must(t.Parse(data))
	}
	{
		t := template.New("section")
		templates.Section = template.Must(t.Parse(data))
	}
	{
		t := template.New("entry")
		templates.Entry = template.Must(t.Parse(data))
	}
	{
		t := template.New("description")
		templates.Description = template.Must(t.Parse(data))
	}
	return templates, nil
}

func NewGalleryTemplate(settings *base.SiteSettings) (*template.Template, error) {
	ti := template.New("thumbnails")
	functions := template.FuncMap{}
	functions["view_author_title"] = view_author_title
	ti = ti.Funcs(functions)
	template_data, data_err := ioutil.ReadFile(
		path.Join(settings.TemplatesDir, "thumbnails.html.tmpl"))
	if data_err != nil {
		return nil, data_err
	}
	t, template_err := ti.Parse(string(template_data))
	if template_err != nil {
		return nil, template_err
	}
	return t, nil
}

func render_template(
	w http.ResponseWriter,
	t *template.Template,
	data interface{}) error {
	wr := bufio.NewWriterSize(w, 1024*64)
	err := t.Execute(wr, data)
	if err != nil {
		return err
	} else {
		return wr.Flush()
	}
	return nil
}

type RequestHandlerFunc func(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request)

func handle_entry(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("%v %s\n", path_elements, r.URL)
	render(w, site.Settings)
}

func handle_section(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("%v %s\n", path_elements, r.URL)
	render(w, site.Settings)
}

func handle_year(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	// fmt.Printf("year %v %s\n", path_elements, r.URL)
	render(w, site.Settings)
}

func _read_year_range(
	site Site, r *http.Request) ([]*base.Year, *InternalLink, *InternalLink, error) {
	year_start := 0
	year_end := 99999
	year_range_requested := string(r.FormValue("y"))
	if len(year_range_requested) > 0 {
		year_range_regexp := regexp.MustCompile("^\\d{4}-\\d{4}$")
		if len(year_range_regexp.FindString(year_range_requested)) == 0 {
			log.Printf("Year range is not a numeric range")
			return nil, nil, nil, errors.New("Year range is not a numeric range")
		}
		start_end_str := strings.Split("-", year_range_requested)
		var err error
		year_start, err = strconv.Atoi(start_end_str[0])
		if err != nil {
			panic(err)
		}
		year_end, err = strconv.Atoi(start_end_str[1])
		if err != nil {
			panic(err)
		}
		if year_end < year_start {
			log.Printf("End year %d < start year %d", year_end, year_start)
			return nil, nil, nil, errors.New("year_end < year_start")
		}
	} else if len(site.State.Years) > 0 {
		// Years array is sorted in the reverse order.
		year_end = site.State.Years[0].Year
		year_start = year_start + DEFAULT_MAIN_YEARS
	}

	return nil, nil, nil, nil
}

func handle_main(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	fmt.Printf("main %v %s\n", path_elements, r.URL)
	years, years_before, years_after, err_year_range := _read_year_range(site, r)
	if err_year_range != nil {
		_bad_request(w)
		log.Printf("Invalid year range request: %s", err_year_range)
		return
	}
	gallery_thumbnails := make([]GalleryThumbnails, len(years))
	for i, year := range years {
		gallery_thumbnails[i] = GalleryThumbnails{
			Path:    year.Path,
			Title:   year.Name,
			Entries: random_select_entries(year, 5),
		}
	}
	page_context := PageContext{}
	context := MainContext{
		Galleries:   gallery_thumbnails,
		YearsBefore: *years_before,
		YearsAfter:  *years_after,
		Context:     page_context,
	}
	err_template := render_template(w, site.Templates.Main, context)
	if err_template != nil {
		server.Ise(w)
		log.Printf("Internal main page error: %s", err_template)
	}
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

func route_request(site Site,
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
			handler.callback(site, path_elements, w, r)
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("NOTFOUND %s\n", r.URL)
		http.NotFound(w, r)
	}
}

func SiteRenderer(settings base.SiteSettings, state *state.SiteState) http.HandlerFunc {
	site := Site{
		Settings:  settings,
		State:     state,
		Templates: nil,
	}

	entry := base.EntryInfo{
		Path:          "/2018/section/entry/",
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
	section_ranked := base.Section{
		Path:        "/2018/section-ranked/",
		Key:         "section-ranked",
		Name:        "Section ranked",
		Description: "Here is a decent ranked description!",
		IsRanked:    true,
		Entries:     []*base.EntryInfo{&entry, &entry, &entry, &entry, &entry, &entry},
	}
	section_unranked := base.Section{
		Path:        "/2018/section-unranked/",
		Key:         "section-unranked",
		Name:        "Section unranked",
		Description: "Here is a decent unranked description!",
		IsRanked:    false,
		Entries:     []*base.EntryInfo{&entry, &entry, &entry, &entry, &entry, &entry},
	}
	year := base.Year{
		Year:     2018,
		Path:     "/2018/",
		Key:      "2018",
		Name:     "2018",
		Sections: []*base.Section{&section_ranked, &section_unranked},
	}
	state.Years = make([]*base.Year, 1)
	state.Years[0] = &year

	return func(w http.ResponseWriter, r *http.Request) {
		route_request(site, w, r)
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
}
