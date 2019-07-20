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
var MAX_PREVIEW_ENTRIES = 5
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

type Breadcrumbs struct {
	Parents []InternalLink
	Last    InternalLink
}

type PageContext struct {
	Path        string
	Breadcrumbs Breadcrumbs
	Title       string
	SiteRoot    string
	Url         string
	CurrentYear int
	Description string
	SiteState   *state.SiteState
	Navigation  *state.Navigable
}

type GalleryThumbnails struct {
	Path    string
	Title   string
	Entries []*base.Entry
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

type YearInfo struct {
	Prev *base.Year
	Curr *base.Year
	Next *base.Year
}

type YearContext struct {
	Year      YearInfo
	Galleries []GalleryThumbnails
	Context   PageContext
}

type SectionInfo struct {
	Year *base.Year
	Prev *base.Section
	Curr *base.Section
	Next *base.Section
}

type EntryInfo struct {
	Year    *base.Year
	Section *base.Section
	Prev    *base.Entry
	Curr    *base.Entry
	Next    *base.Entry
}

type SectionContext struct {
	Year    *base.Year
	Section SectionInfo
	Context PageContext
}

type EntryContext struct {
	Year    *base.Year
	Section *base.Section
	Entry   EntryInfo
	Context PageContext
}

func in_array(array []*base.Entry, entry *base.Entry) bool {
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

func random_select_section_entries(section *base.Section, amount int) []*base.Entry {
	section_indexes := rand.Perm(len(section.Entries))
	max_items := len(section.Entries)
	if amount < max_items {
		max_items = amount
	}
	result := make([]*base.Entry, max_items)
	for i := 0; i < max_items; i++ {
		result[i] = section.Entries[section_indexes[i]]
	}
	return result
}

// Randomly selects a number of entries by taking limited amount of
// entries from each section. There are basically many different
// possibilities to select viewable entries for the main page, but
// this is a simple unweighted logic that makes sure that one section
// with hundreds of entries does not dominate.
func random_select_entries(year *base.Year, amount int) []*base.Entry {
	total_sections := len(year.Sections)
	section_indexes := rand.Perm(total_sections * MAX_MAIN_SECTION_ENTRIES)
	var result []*base.Entry
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
func peek_section_entries(section base.Section, amount int) []*base.Entry {
	if section.IsRanked {
		return section.Entries[:amount]
	}

	var result []*base.Entry
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

func view_author_title(entry base.Entry) string {
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

func load_template(
	settings *base.SiteSettings,
	name string,
	filename string,
	clonable *template.Template) (*template.Template, error) {
	t := template.New(name)
	if clonable != nil {
		cloned, err := clonable.Clone()
		if err != nil {
			return nil, err
		}
		t = cloned.New(name)
	}
	template_data, data_err := ioutil.ReadFile(
		path.Join(settings.TemplatesDir, filename))
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
		generic = template.Must(
			load_template(settings, "thumbnails", "thumbnails.html.tmpl", generic))
		generic = template.Must(
			load_template(settings, "breadcrumbs", "breadcrumbs.html.tmpl", generic))
		generic = template.Must(
			load_template(settings, "navbar", "navbar.html.tmpl", generic))
	}
	{
		contents := template.Must(
			load_template(settings, "page-contents", "main.html.tmpl", generic))
		templates.Main, err = load_template(
			settings, "main", "layout.html.tmpl", contents)
		if err != nil {
			return templates, err
		}
	}
	{
		contents := template.Must(
			load_template(settings, "page-contents", "year.html.tmpl", generic))
		templates.Year, err = load_template(
			settings, "year", "layout.html.tmpl", contents)
		if err != nil {
			return templates, err
		}
	}
	{
		contents := template.Must(
			load_template(settings, "page-contents", "section.html.tmpl", generic))
		templates.Section, err = load_template(
			settings, "section", "layout.html.tmpl", contents)
		if err != nil {
			return templates, err
		}
	}
	{
		contents := template.Must(
			load_template(settings, "page-contents", "entry.html.tmpl", generic))
		templates.Entry, err = load_template(
			settings, "entry", "layout.html.tmpl", contents)
		if err != nil {
			return templates, err
		}
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
	entry, err_info := get_entry_info(site, path_elements)
	if err_info != nil {
		log.Println(err_info)
		http.NotFound(w, r)
		return
	}

	page_context := PageContext{
		Path:     path_elements[""],
		Title:    view_author_title(*entry.Curr),
		SiteRoot: site.Settings.SiteRoot,
		Breadcrumbs: Breadcrumbs{
			Parents: []InternalLink{
				InternalLink{
					Path:     entry.Year.Path,
					Contents: entry.Year.Key,
				},
				InternalLink{
					Path:     entry.Section.Path,
					Contents: entry.Section.Name,
				},
			},
		},
	}
	context := EntryContext{
		Year:    entry.Year,
		Section: entry.Section,
		Entry:   entry,
		Context: page_context,
	}
	err_template := render_template(w, site.Templates.Entry, context)
	if err_template != nil {
		server.Ise(w)
		log.Printf("Internal entry page error: %s", err_template)
	}
}

func handle_section(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	section, err_info := get_section_info(site, path_elements)
	if err_info != nil {
		log.Println(err_info)
		http.NotFound(w, r)
		return
	}

	title := section.Year.Key + " / " + section.Curr.Name
	page_context := PageContext{
		Path:     path_elements[""],
		Title:    title,
		SiteRoot: site.Settings.SiteRoot,
		Breadcrumbs: Breadcrumbs{
			Parents: []InternalLink{
				InternalLink{
					Path:     section.Year.Path,
					Contents: section.Year.Key,
				},
			},
			Last: InternalLink{
				Path:     section.Curr.Path,
				Contents: section.Curr.Name,
			},
		},
	}
	context := SectionContext{
		Section: section,
		Context: page_context,
	}
	err_template := render_template(w, site.Templates.Section, context)
	if err_template != nil {
		server.Ise(w)
		log.Printf("Internal main page error: %s", err_template)
	}
}

func get_year_info(site Site, path_elements map[string]string) (YearInfo, error) {
	info := YearInfo{}
	requested_year, year_err := strconv.Atoi(path_elements["Year"])
	if year_err != nil {
		return info, errors.New(
			fmt.Sprintf(
				"Invalid requested year: %s: %s",
				path_elements["Year"],
				year_err))
	}
	last_index := 0
	for i, candidate_year := range site.State.Years {
		last_index = i
		if candidate_year.Year == requested_year {
			info.Curr = candidate_year
			break
		}
		info.Next = candidate_year
	}
	if info.Curr == nil {
		return info, errors.New(
			fmt.Sprintf("Year %s not found!", path_elements["Year"]))
	}
	if last_index+1 < len(site.State.Years) {
		info.Prev = site.State.Years[last_index+1]
	}
	return info, nil
}

func get_section_info(site Site, path_elements map[string]string) (SectionInfo, error) {
	info := SectionInfo{}
	year, year_err := get_year_info(site, path_elements)
	if year_err != nil {
		return info, year_err
	}
	info.Year = year.Curr
	key := path_elements["Section"]
	last_index := 0
	for i, candidate := range year.Curr.Sections {
		last_index = i
		if candidate.Key == key {
			info.Curr = candidate
			break
		}
		info.Next = candidate
	}
	if info.Curr == nil {
		return info, errors.New(
			fmt.Sprintf("Section %s not found!", key))
	}
	if last_index+1 < len(year.Curr.Sections) {
		info.Prev = year.Curr.Sections[last_index+1]
	}
	return info, nil
}

func get_entry_info(site Site, path_elements map[string]string) (EntryInfo, error) {
	info := EntryInfo{}
	section, info_err := get_section_info(site, path_elements)
	if info_err != nil {
		return info, info_err
	}
	info.Year = section.Year
	info.Section = section.Curr
	key := path_elements["Entry"]
	last_index := 0
	for i, candidate := range section.Curr.Entries {
		last_index = i
		if candidate.Key == key {
			info.Curr = candidate
			break
		}
		info.Next = candidate
	}
	if info.Curr == nil {
		return info, errors.New(
			fmt.Sprintf("Entry %s not found!", key))
	}
	if last_index+1 < len(section.Curr.Entries) {
		info.Prev = section.Curr.Entries[last_index+1]
	}
	return info, nil
}

func handle_year(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	year, err_info := get_year_info(site, path_elements)
	if err_info != nil {
		log.Println(err_info)
		http.NotFound(w, r)
		return
	}

	page_context := PageContext{
		Path:     path_elements[""],
		Title:    year.Curr.Key,
		SiteRoot: site.Settings.SiteRoot,
		Breadcrumbs: Breadcrumbs{
			Last: InternalLink{
				Path:     year.Curr.Path,
				Contents: year.Curr.Key,
			},
		},
	}

	gallery_thumbnails := make([]GalleryThumbnails, len(year.Curr.Sections))
	for i, section := range year.Curr.Sections {
		thumbnails := GalleryThumbnails{
			Path:    section.Path,
			Title:   section.Name,
			Entries: random_select_section_entries(section, MAX_PREVIEW_ENTRIES),
		}
		gallery_thumbnails[i] = thumbnails
	}
	context := YearContext{
		Galleries: gallery_thumbnails,
		Year:      year,
		Context:   page_context,
	}
	err_template := render_template(w, site.Templates.Year, context)
	if err_template != nil {
		server.Ise(w)
		log.Printf("Internal main page error: %s", err_template)
	}
}

func _create_year_range_link(
	site Site,
	years []*base.Year,
	latest_year *base.Year) InternalLink {
	if len(years) == 0 {
		return InternalLink{}
	}
	if len(years) == 1 {
		return InternalLink{
			Path:     fmt.Sprintf("/?y=%d-%d", years[0].Year, years[0].Year),
			Contents: fmt.Sprintf("%d", years[0].Year),
		}
	}
	years_first := years[len(years)-1]
	years_last := years[0]
	if years_last == latest_year {
		return InternalLink{
			Path:     "/",
			Contents: fmt.Sprintf("%d-%d", years_first.Year, years_last.Year),
		}
	}
	return InternalLink{
		Path:     fmt.Sprintf("/?y=%d-%d", years_first.Year, years_last.Year),
		Contents: fmt.Sprintf("%d-%d", years_first.Year, years_last.Year),
	}
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

	var latest_year *base.Year
	if len(site.State.Years) > 0 {
		latest_year = site.State.Years[0]
	}
	link_before := _create_year_range_link(site, years_before, latest_year)
	link_after := _create_year_range_link(site, years_after, latest_year)
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
			Entries: random_select_entries(year, MAX_PREVIEW_ENTRIES),
		}
	}
	page_context := PageContext{
		Path:     path_elements[""],
		SiteRoot: site.Settings.SiteRoot,
	}
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
