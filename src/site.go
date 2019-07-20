package site

import (
	"base"
	"bufio"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"regexp"
	"server"
	"state"
	"strconv"
	"text/template"
)

var DEFAULT_MAIN_YEARS = 15
var MAX_MAIN_ENTRIES = 5
var MAX_MAIN_SECTION_ENTRIES = 2

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

// Randomly selects a number of entries by taking limited amount of
// entries from each section. There are basically many different
// possibilities to select viewable entries for the main page, but
// this is a simple unweighted logic that makes sure that one section
// with hundreds of entries does not dominate.
func random_select_entries(year *base.Year, amount int) []*base.EntryInfo {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * MAX_MAIN_SECTION_ENTRIES)
	var result []*base.EntryInfo
	for _, index_value := range section_indexes {
		if len(result) == amount {
			break
		}
		index := index_value % total_sections
		entries := year.Sections[index].Entries
		entry := entries[rand.Intn(len(entries))]
		// Technically it's possible that we only get 1
		// entry/section. That's not a problem.
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

func load_template(settings *base.SiteSettings, name string, clonable *template.Template) (*template.Template, error) {
	t := template.New(name)
	if clonable != nil {
		t = clonable.New(name)
	}
	template_data, data_err := ioutil.ReadFile(
		path.Join(settings.TemplatesDir, name+".html.tmpl"))
	if data_err != nil {
		return nil, data_err
	}
	t_read, parse_err := t.Parse(string(template_data))
	if parse_err != nil {
		return nil, parse_err
	}
	return t_read, nil
}

func create_base_template(name string) *template.Template {
	t := template.New(name)
	functions := template.FuncMap{}
	functions["view_author_title"] = view_author_title
	return t.Funcs(functions)
}

func load_templates(settings *base.SiteSettings) (SiteTemplates, error) {
	var templates SiteTemplates
	data := "asdf"

	generic := create_base_template("generic")

	var err error
	{
		generic, err = load_template(settings, "thumbnails", generic)
		if err != nil {
			return templates, err
		}
	}
	{
		templates.Main, err = load_template(settings, "main", generic)
		if err != nil {
			return templates, err
		}
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
}

func handle_section(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("%v %s\n", path_elements, r.URL)
}

func handle_year(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	// fmt.Printf("year %v %s\n", path_elements, r.URL)
}

func _create_year_range_link(years []*base.Year) InternalLink {
	if len(years) > 1 {
		years_first := years[len(years)-1]
		years_last := years[0]
		return InternalLink{
			Path:     fmt.Sprintf("?y=%d-%d", years_first.Year, years_last.Year),
			Contents: fmt.Sprintf("%d-%d", years_first.Year, years_last.Year),
		}
	} else if len(years) == 1 {
		return InternalLink{
			Path:     fmt.Sprintf("?y=%d-%d", years[0].Year, years[0].Year),
			Contents: fmt.Sprintf("%d", years[0].Year),
		}
	}
	return InternalLink{}
}

func _read_year_range(
	site Site, r *http.Request) ([]*base.Year, *InternalLink, *InternalLink, error) {
	year_start := 0
	year_end := 99999
	year_range_requested := string(r.FormValue("y"))
	if len(year_range_requested) > 0 {
		year_range_regexp := regexp.MustCompile("^(\\d{4})-(\\d{4})$")
		match := year_range_regexp.FindStringSubmatch(year_range_requested)
		if match == nil {
			log.Printf("Year range is not a numeric range")
			return nil, nil, nil, errors.New("Year range is not a numeric range")
		}
		var err error
		year_start, err = strconv.Atoi(match[1])
		if err != nil {
			panic(err)
		}
		year_end, err = strconv.Atoi(match[2])
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
		year_start = year_end - DEFAULT_MAIN_YEARS
	}
	max_year := year_end + 1 + DEFAULT_MAIN_YEARS
	min_year := year_start - 1 - DEFAULT_MAIN_YEARS

	var years_before []*base.Year
	var years []*base.Year
	var years_after []*base.Year
	for _, year := range site.State.Years {
		if max_year < year.Year {
			continue
		}
		if year.Year < min_year {
			continue
		}
		if year.Year < year_start {
			years_before = append(years_before, year)
		} else if year_end < year.Year {
			years_after = append(years_after, year)
		} else {
			years = append(years, year)
		}
	}

	link_before := _create_year_range_link(years_before)
	link_after := _create_year_range_link(years_after)
	return years, &link_before, &link_after, nil
}

func handle_main(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	//fmt.Printf("main %v %s\n", path_elements, r.URL)
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
			Entries: random_select_entries(year, MAX_MAIN_ENTRIES),
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
	templates, err := load_templates(&settings)
	if err != nil {
		log.Println(err)
		panic("Unable to load templates!")
	}
	site := Site{
		Settings:  settings,
		State:     state,
		Templates: &templates,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		route_request(site, w, r)
	}
}
