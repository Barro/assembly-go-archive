package site

import (
	"base"
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"server"
	"state"
	"strconv"
	"strings"
	"text/template"
)

var DEFAULT_MAIN_YEARS = 15
var MAX_SECTION_DISPLAY_ENTRIES = 30
var MAX_PREVIEW_ENTRIES = 5
var MAX_MAIN_SECTION_ENTRIES = 2

var YEARLY_NAVIGATION_YEARS = 7

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
	Static    map[string]string
}

type YearlyNavigation struct {
	Years        []InternalLink
	CurrentIndex int
}

type Breadcrumbs struct {
	Parents []InternalLink
	Last    InternalLink
}

type PageNavigation struct {
	Prev InternalLink
	Next InternalLink
}

type PageContext struct {
	Path        string
	Breadcrumbs Breadcrumbs
	Title       string
	SiteRoot    string
	Static      map[string]string
	CurrentYear int
	Description string
	// Prefertches are used on gallery like pages to fetch the next
	// and previous possible pages. They must point to resources that
	// can be cached, so these are only applicable for sections and
	// entries.
	Prefetches       []string
	SiteState        *state.SiteState
	Navigation       PageNavigation
	YearlyNavigation YearlyNavigation
}

type GalleryThumbnails struct {
	Path    string
	Title   string
	Entries []*base.Entry
}

type InternalLink struct {
	Path     string
	Contents string
	Title    string
}

type MainContext struct {
	Galleries   []GalleryThumbnails
	YearsBefore InternalLink
	YearsAfter  InternalLink
	Context     PageContext
}

type YearInfo struct {
	Prev base.Year
	Curr base.Year
	Next base.Year
}

type YearContext struct {
	Year      YearInfo
	Galleries []GalleryThumbnails
	Context   PageContext
}

type SectionInfo struct {
	Year *base.Year
	Prev base.Section
	Curr base.Section
	Next base.Section
}

type EntryInfo struct {
	Year    *base.Year
	Section *base.Section
	Prev    base.Entry
	Curr    base.Entry
	Next    base.Entry
}

type DisplayEntries struct {
	Row     int
	Entries []*base.Entry
}

type SectionContext struct {
	Year             *base.Year
	Section          SectionInfo
	DisplayEntries   []*base.Entry
	OffsetNavigation PageNavigation
	Context          PageContext
}

type EntryContext struct {
	Year    *base.Year
	Section *base.Section
	Entry   EntryInfo
	Asset   string
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

func view_get_image_data_src(image base.ImageInfo) string {
	data, err := ioutil.ReadFile(image.FsPath)
	// This should basically lead into 404 error:
	if err != nil {
		return fmt.Sprintf(
			"%s?%s", html.EscapeString(image.Path), image.Checksum)
	}
	return fmt.Sprintf(
		"data:image/%s;base64,%s",
		image.Type,
		base64.StdEncoding.EncodeToString(data))
}

func struct_display_entries(row int, thumbnails []*base.Entry) DisplayEntries {
	return DisplayEntries{
		Row:     row,
		Entries: thumbnails,
	}
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
		if len(entries) == 0 {
			continue
		}
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

// Adds a short cache control header value. This is used for pages
// that can change, but will very likely not change unless the site
// layout or page contents is updated.
func add_short_cache_time(w http.ResponseWriter) {
	header := w.Header()
	header.Add("Cache-Control", "public, max-age=60")
}

func author_title(entry base.Entry) string {
	var author_title string
	if entry.Title == "" {
		return ""
	} else if entry.Author == "" {
		author_title = entry.Title
	} else {
		author_title = entry.Title + " by " + entry.Author
	}
	return author_title
}

func view_author_title(entry base.Entry) string {
	return html.EscapeString(author_title(entry))
}

func view_cut_string(target string, max_length int) string {
	MAX_WORD_LENGTH := 23
	words := regexp.MustCompile("(\\w+)|(\\W+)").FindAllString(target, -1)
	var short_words []string
	for _, word := range words {
		for len(word) > MAX_WORD_LENGTH {
			short_words = append(short_words, word[:MAX_WORD_LENGTH])
			short_words = append(short_words, "\u200B")
			word = word[MAX_WORD_LENGTH:]
		}
		short_words = append(short_words, word)
	}
	word_cut_data := strings.Join(short_words, "")
	if len(word_cut_data) < max_length {
		return word_cut_data
	}
	return strings.TrimSpace(word_cut_data[:max_length-3]) + "\u2026"
}

func view_attribute(name string, value string) string {
	if len(value) == 0 {
		return ""
	}
	return name + "=\"" + html.EscapeString(value) + "\""
}

func view_image_srcset(images []base.ImageInfo) string {
	type SrcSet struct {
		Srcs  []string
		Sizes []string
	}
	sets := map[string]*SrcSet{}
	for _, image := range images {
		srcset, ok := sets[image.Type]
		if !ok {
			srcset = &SrcSet{}
			sets[image.Type] = srcset
		}
		srcset.Srcs = append(
			srcset.Srcs,
			fmt.Sprintf(
				"%s?%s %dw",
				html.EscapeString(image.Path),
				html.EscapeString(image.Checksum),
				image.Size.X))
		srcset.Sizes = append(
			srcset.Sizes, fmt.Sprintf("%dpx", image.Size.X))
	}
	result := ""
	for set_type, set_value := range sets {
		result += fmt.Sprintf(
			"<source type='%s' srcset='%s' sizes='%s' />",
			set_type,
			strings.Join(set_value.Srcs, ", "),
			strings.Join(set_value.Sizes, ", "),
		)
	}
	return result
}

func add_prefetch_links(context *PageContext) {
	var result []string
	if len(context.Navigation.Prev.Path) > 0 {
		result = append(result, context.Navigation.Prev.Path)
	}
	if len(context.Navigation.Next.Path) > 0 {
		result = append(result, context.Navigation.Next.Path)
	}
	context.Prefetches = result
}

func mod_context_no_breadcrumbs(context PageContext) PageContext {
	no_breadcrumbs := context
	no_breadcrumbs.Breadcrumbs = Breadcrumbs{}
	return no_breadcrumbs
}

func mod_context_replace_navigation(context PageContext, navigation PageNavigation) PageContext {
	replaced := context
	replaced.Navigation = navigation
	return replaced
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
	functions["view_cut_string"] = view_cut_string
	functions["view_attribute"] = view_attribute
	functions["mod_context_no_breadcrumbs"] = mod_context_no_breadcrumbs
	functions["mod_context_replace_navigation"] = mod_context_replace_navigation
	functions["view_get_image_data_src"] = view_get_image_data_src
	functions["struct_display_entries"] = struct_display_entries
	functions["view_image_srcset"] = view_image_srcset
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
		generic = template.Must(
			load_template(settings, "yearlynavigation", "yearlynavigation.html.tmpl", generic))
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

func walk_directories_impl(root string, walkFn filepath.WalkFunc, subpath string, current_depth int, err error) error {
	if err != nil {
		return err
	}
	if current_depth > 8 {
		return errors.New(
			fmt.Sprintf(
				"Trying to recurse too deep under %s at %s!", root, subpath))
	}
	files, err_readdir := ioutil.ReadDir(path.Join(root, subpath))
	if err_readdir != nil {
		return err_readdir
	}
	for _, info := range files {
		new_subpath := path.Join(subpath, info.Name())
		if info.IsDir() {
			err_walk := walk_directories_impl(
				root, walkFn, new_subpath, current_depth+1, nil)
			if err_walk != nil {
				return err_walk
			}
			continue
		}
		if err := walkFn(new_subpath, info, nil); err != nil {
			return err
		}
	}
	return nil
}

func walk_directories(root string, walkFn filepath.WalkFunc) error {
	return walk_directories_impl(root, walkFn, "", 0, nil)
}

func load_static_files(settings *base.SiteSettings) (map[string]string, error) {
	result := make(map[string]string)
	err := walk_directories(
		settings.StaticDir,
		func(subpath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fs_path := path.Join(settings.StaticDir, subpath)
			checksum, err_checksum := base.CreateFileChecksum(fs_path)
			if err_checksum != nil {
				log.Printf("Checksum calculation failed on %s", fs_path)
				return err_checksum
			}
			result[subpath] = checksum
			return nil
		},
	)
	return result, err
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

type AssetHandler func(site Site, entry base.Entry) string

var ASSET_HANDLERS = map[string]AssetHandler{
	"youtube": handle_asset_youtube,
	"image":   handle_asset_image,
}

func handle_asset_youtube(site Site, entry base.Entry) string {
	youtube := entry.Asset.Data.(state.YoutubeAsset)
	EMBED_TEMPLATE := `<iframe id="ytplayerembed" class="youtube-player" width="%d" height="%d" src="https://www.youtube.com/embed/%s" style="border: 0px" allowfullscreen="allowfullscreen">\n</iframe>`
	CONTROLS_HEIGHT := 0.0
	ASPECT_RATIO := 16.0 / 9.0
	DEFAULT_WIDTH := 640
	width := DEFAULT_WIDTH
	height := int(float64(width)/ASPECT_RATIO + CONTROLS_HEIGHT)
	embed_id := youtube.Id
	if strings.Contains(youtube.Id, "#t=") {
		splits := strings.SplitN(youtube.Id, "#t=", 2)
		id := splits[0]
		timestamp := splits[1]
		embed_id = fmt.Sprintf("%s?start=%s", id, timestamp)
	}
	return fmt.Sprintf(EMBED_TEMPLATE, width, height, html.EscapeString(embed_id))
}

func handle_asset_image(site Site, entry base.Entry) string {
	image := entry.Asset.Data.(state.ImageAsset)
	EMBED_TEMPLATE := `
<picture>
    %s
    <img src="%s" alt="%s" title="%s" width="%d" height="%d" />
</picture>
`
	image_path := fmt.Sprintf(
		"%s?%s",
		image.Default.Path,
		image.Default.Checksum)
	image_author_title := author_title(entry)
	return fmt.Sprintf(
		EMBED_TEMPLATE,
		view_image_srcset(image.Sources),
		html.EscapeString(image_path),
		html.EscapeString(image_author_title),
		html.EscapeString(image_author_title),
		image.Default.Size.X,
		image.Default.Size.Y,
	)
}

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
		Title:    author_title(entry.Curr),
		SiteRoot: site.Settings.SiteRoot,
		Static:   site.Static,
		Breadcrumbs: Breadcrumbs{
			Parents: []InternalLink{
				InternalLink{
					Path:     entry.Year.Path,
					Contents: entry.Year.Key,
				},
				InternalLink{
					Path:     entry.Section.Path,
					Contents: entry.Section.Name,
					Title:    entry.Section.Name,
				},
			},
		},
		YearlyNavigation: get_yearly_navigation(site, entry.Year.Year),
		CurrentYear:      entry.Year.Year,
		Navigation: PageNavigation{
			Prev: InternalLink{
				Path:     entry.Prev.Path,
				Contents: author_title(entry.Prev),
				Title:    author_title(entry.Prev),
			},
			Next: InternalLink{
				Path:     entry.Next.Path,
				Contents: author_title(entry.Next),
				Title:    author_title(entry.Next),
			},
		},
	}

	asset_handler, ok := ASSET_HANDLERS[entry.Curr.Asset.Type]
	if !ok {
		server.Ise(w)
		log.Printf(
			"No handler on %s for asset type %s",
			entry.Curr.Path,
			entry.Curr.Asset.Type)
		return
	}
	context := EntryContext{
		Year:    entry.Year,
		Section: entry.Section,
		Entry:   entry,
		Asset:   asset_handler(site, entry.Curr),
		Context: page_context,
	}
	add_prefetch_links(&context.Context)
	// Entry can be cached for a short while, as there is no
	// randomness in it.
	add_short_cache_time(w)
	err_template := render_template(w, site.Templates.Entry, context)
	if err_template != nil {
		server.Ise(w)
		log.Printf("Internal entry page error: %s", err_template)
		return
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

	offset_str := string(r.FormValue("offset"))
	if offset_str == "" {
		offset_str = "0"
	}
	offset, err_offset := strconv.Atoi(offset_str)
	if err_offset != nil {
		log.Println(err_offset)
		offset = 0
	} else if offset < 0 {
		offset = 0
	} else if offset%MAX_SECTION_DISPLAY_ENTRIES != 0 {
		offset = 0
	}

	prev_offset := offset - MAX_SECTION_DISPLAY_ENTRIES
	if prev_offset < 0 {
		prev_offset = 0
	}
	next_offset := offset + MAX_SECTION_DISPLAY_ENTRIES

	var display_entries []*base.Entry
	total_entries := len(section.Curr.Entries)
	if total_entries <= MAX_SECTION_DISPLAY_ENTRIES {
		display_entries = section.Curr.Entries
	} else {
		if offset+MAX_SECTION_DISPLAY_ENTRIES < total_entries {
			display_entries = section.Curr.Entries[offset:(offset + MAX_SECTION_DISPLAY_ENTRIES)]
		} else if offset < total_entries {
			display_entries = section.Curr.Entries[offset:total_entries]
		}
	}
	var offset_navigation PageNavigation
	if (prev_offset == 0 && offset != 0) || prev_offset > 0 {
		offset_navigation.Next = InternalLink{
			Contents: "Previous " + strconv.Itoa(MAX_SECTION_DISPLAY_ENTRIES) + " items",
			Path:     "?offset=" + strconv.Itoa(prev_offset),
		}
		if prev_offset == 0 {
			offset_navigation.Next.Path = site.Settings.SiteRoot + "/" + path_elements[""]
		}
	}
	if next_offset < total_entries {
		next_entries_count := total_entries - next_offset
		if MAX_SECTION_DISPLAY_ENTRIES < next_entries_count {
			next_entries_count = MAX_SECTION_DISPLAY_ENTRIES
		}
		offset_navigation.Prev = InternalLink{
			Contents: "Next " + strconv.Itoa(next_entries_count) + " items",
			Path:     "?offset=" + strconv.Itoa(next_offset),
		}
	}

	title := section.Year.Key + " / " + section.Curr.Name
	page_context := PageContext{
		Path:     path_elements[""],
		Title:    title,
		SiteRoot: site.Settings.SiteRoot,
		Static:   site.Static,
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
				Title:    section.Curr.Name,
			},
		},
		YearlyNavigation: get_yearly_navigation(site, section.Year.Year),
		CurrentYear:      section.Year.Year,
		Navigation: PageNavigation{
			Prev: InternalLink{
				Path:     section.Prev.Path,
				Contents: section.Prev.Name,
				Title:    section.Prev.Name,
			},
			Next: InternalLink{
				Path:     section.Next.Path,
				Contents: section.Next.Name,
				Title:    section.Next.Name,
			},
		},
	}
	context := SectionContext{
		DisplayEntries:   display_entries,
		OffsetNavigation: offset_navigation,
		Section:          section,
		Context:          page_context,
	}
	add_prefetch_links(&context.Context)
	// Section can be cached for a short while, as there is no
	// randomness in it.
	add_short_cache_time(w)
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
			info.Curr = *candidate_year
			break
		}
		info.Next = *candidate_year
	}
	if info.Curr.Key == "" {
		return info, errors.New(
			fmt.Sprintf("Year %s not found!", path_elements["Year"]))
	}
	if last_index+1 < len(site.State.Years) {
		info.Prev = *site.State.Years[last_index+1]
	}
	return info, nil
}

func get_section_info(site Site, path_elements map[string]string) (SectionInfo, error) {
	info := SectionInfo{}
	year, year_err := get_year_info(site, path_elements)
	if year_err != nil {
		return info, year_err
	}
	info.Year = &year.Curr
	key := path_elements["Section"]
	last_index := 0
	for i, candidate := range year.Curr.Sections {
		last_index = i
		if candidate.Key == key {
			info.Curr = *candidate
			break
		}
		info.Next = *candidate
	}
	if info.Curr.Key == "" {
		return info, errors.New(
			fmt.Sprintf("Section %s not found!", key))
	}
	if last_index+1 < len(year.Curr.Sections) {
		info.Prev = *year.Curr.Sections[last_index+1]
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
	info.Section = &section.Curr
	key := path_elements["Entry"]
	last_index := 0
	for i, candidate := range section.Curr.Entries {
		last_index = i
		if candidate.Key == key {
			info.Curr = *candidate
			break
		}
		info.Next = *candidate
	}
	if info.Curr.Key == "" {
		return info, errors.New(
			fmt.Sprintf("Entry %s not found!", key))
	}
	if last_index+1 < len(section.Curr.Entries) {
		info.Prev = *section.Curr.Entries[last_index+1]
	}
	return info, nil
}

func get_yearly_navigation(site Site, current_year int) YearlyNavigation {
	if len(site.State.Years) == 0 {
		return YearlyNavigation{}
	}
	year_max := site.State.Years[0].Year
	year_min := site.State.Years[len(site.State.Years)-1].Year
	years_count := len(site.State.Years)
	highlighted_index := -1
	if current_year < year_min {
		highlighted_index = -1
	} else if year_max < current_year {
		highlighted_index = -1
	} else {
		highlighted_index = year_max - current_year
	}

	visible_years := YEARLY_NAVIGATION_YEARS
	if highlighted_index < int(YEARLY_NAVIGATION_YEARS/2)+2 {
		visible_years++
	}
	if years_count-highlighted_index < int(YEARLY_NAVIGATION_YEARS/2)+1 {
		visible_years++
	}

	index_first := highlighted_index - visible_years/2
	index_last := index_first + visible_years
	if index_last >= years_count {
		index_last = years_count
		index_first = index_last - visible_years
	}
	if index_first < 0 {
		index_first = 0
		index_last = index_first + visible_years
		if index_last >= years_count {
			index_last = years_count
		}
	}
	var display_years []InternalLink
	if 0 < index_first {
		laquo := InternalLink{
			Path:     site.State.Years[index_first-1].Path,
			Contents: "«",
			Title:    site.State.Years[index_first-1].Key,
		}
		display_years = append(display_years, laquo)
	}

	current_index := -1
	for i := index_first; i < index_last; i++ {
		year_link := InternalLink{
			Path:     site.State.Years[i].Path,
			Contents: fmt.Sprintf("'%02d", (site.State.Years[i].Year % 100)),
			Title:    site.State.Years[i].Key,
		}
		if i == highlighted_index {
			current_index = len(display_years)
		}
		display_years = append(display_years, year_link)
	}

	if index_last < years_count {
		raquo := InternalLink{
			Path:     site.State.Years[index_last].Path,
			Contents: "»",
			Title:    site.State.Years[index_last].Key,
		}
		display_years = append(display_years, raquo)
	}

	return YearlyNavigation{
		Years:        display_years,
		CurrentIndex: current_index,
	}
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
		Static:   site.Static,
		Breadcrumbs: Breadcrumbs{
			Last: InternalLink{
				Path:     year.Curr.Path,
				Contents: year.Curr.Key,
			},
		},
		YearlyNavigation: get_yearly_navigation(site, year.Curr.Year),
		CurrentYear:      year.Curr.Year,
		Navigation: PageNavigation{
			Prev: InternalLink{
				Path:     year.Prev.Path,
				Contents: year.Prev.Key,
			},
			Next: InternalLink{
				Path:     year.Next.Path,
				Contents: year.Next.Key,
			},
		},
	}

	gallery_thumbnails := make([]GalleryThumbnails, len(year.Curr.Sections))
	for i, section := range year.Curr.Sections {
		var display_entries []*base.Entry
		if section.IsRanked && !section.IsOngoing {
			preview_entries := MAX_PREVIEW_ENTRIES
			if len(section.Entries) < preview_entries {
				preview_entries = len(section.Entries)
			}
			display_entries = section.Entries[:preview_entries]
		} else {
			display_entries = random_select_section_entries(section, MAX_PREVIEW_ENTRIES)
		}
		thumbnails := GalleryThumbnails{
			Path:    section.Path,
			Title:   section.Name,
			Entries: display_entries,
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
			Path: fmt.Sprintf(
				"%s/?y=%d", site.Settings.SiteRoot, years[0].Year),
			Contents: fmt.Sprintf("%d", years[0].Year),
		}
	}
	years_first := years[len(years)-1]
	years_last := years[0]
	if years_last == latest_year {
		return InternalLink{
			Path: fmt.Sprintf("%s/", site.Settings.SiteRoot),
			Contents: fmt.Sprintf(
				"%d-%d", years_first.Year, years_last.Year),
		}
	}
	return InternalLink{
		Path: fmt.Sprintf(
			"%s/?y=%d", site.Settings.SiteRoot, years_first.Year),
		Contents: fmt.Sprintf("%d-%d", years_first.Year, years_last.Year),
	}
}

func _read_year_range(
	site Site, r *http.Request) ([]*base.Year, *InternalLink, *InternalLink) {
	year_start := 0
	year_end := 99999
	year_start_requested := string(r.FormValue("y"))
	if len(year_start_requested) > 0 {
		var err error
		year_start, err = strconv.Atoi(year_start_requested)
		if err == nil {
			year_end = year_start + DEFAULT_MAIN_YEARS
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
	return years, &link_before, &link_after
}

func handle_main(
	site Site,
	path_elements map[string]string,
	w http.ResponseWriter,
	r *http.Request) {
	years, years_before, years_after := _read_year_range(site, r)
	gallery_thumbnails := make([]GalleryThumbnails, len(years))
	for i, year := range years {
		gallery_thumbnails[i] = GalleryThumbnails{
			Path:    year.Path,
			Title:   year.Name,
			Entries: random_select_entries(year, MAX_PREVIEW_ENTRIES),
		}
	}
	breadcrumbs_last := ""
	if len(years) > 0 {
		breadcrumbs_last = fmt.Sprintf(
			"%d-%d", years[len(years)-1].Year, years[0].Year)
	}
	page_context := PageContext{
		Path:     path_elements[""],
		SiteRoot: site.Settings.SiteRoot,
		Static:   site.Static,
		Navigation: PageNavigation{
			Prev: *years_before,
			Next: *years_after,
		},
		Breadcrumbs: Breadcrumbs{
			Last: InternalLink{
				Path:     "",
				Contents: breadcrumbs_last,
			},
		},
		YearlyNavigation: get_yearly_navigation(site, 0),
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
	templates, err_templates := load_templates(&settings)
	if err_templates != nil {
		log.Println(err_templates)
		panic("Unable to load templates!")
	}

	static, err_static := load_static_files(&settings)
	if err_static != nil {
		log.Println(err_static)
		panic("Unable to load static files!")
	}

	site := Site{
		Settings:  settings,
		State:     state,
		Templates: &templates,
		Static:    static,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		route_request(site, w, r)
	}
}
