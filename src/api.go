package api

import (
	"archive/tar"
	"base"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"state"
	"strconv"
	"strings"
)

// 128 kilobytes is able to hold 2000 entries with 50 bytes/entry +
// some extra. We should never be even close to this metadata size for
// any year, section, or entry.
var MAX_METADATA_SIZE int64 = 128 * 1024

type ExtractError struct {
	message string
}

func (error *ExtractError) Error() string {
	return error.message
}

type InputError struct {
	message string
}

func (error *InputError) Error() string {
	return error.message
}

func _ise(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error!\n"))
	log.Panic(err)
}

func bad_request(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message + "\n"))
}

type ResolutionError struct {
	message string
}

func (error *ResolutionError) Error() string {
	return error.message
}

func string_to_resolution(value string) (base.Resolution, error) {
	parts := strings.SplitN(value, "x", 2)
	err_res := base.Resolution{}
	if len(parts) != 2 {
		return err_res, &ResolutionError{"Resolution should be in ##x## format!"}
	}
	x, err_x := strconv.Atoi(parts[0])
	if err_x != nil {
		return err_res, err_x
	}
	if x < 1 {
		return err_res, &ResolutionError{"X resolution " + string(x) + " should be positive"}
	}
	y, err_y := strconv.Atoi(parts[1])
	if err_y != nil {
		return err_res, err_y
	}
	if y < 1 {
		return err_res, &ResolutionError{"Y resolution " + string(y) + " should be positive"}
	}
	return base.Resolution{X: x, Y: y}, nil
}

func extract_tar_entry(target string, tar_reader *tar.Reader, header *tar.Header) error {
	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, 0755); err != nil {
			return &ExtractError{"Failed to create directory '" + target + "': " + err.Error()}
		}
	case tar.TypeReg:
		parent_directory := filepath.Dir(target)
		if err := os.MkdirAll(parent_directory, 0755); err != nil {
			return &ExtractError{"Failed to create parent directory for '" + target + "': " + err.Error()}
		}

		out_file, err_create := os.Create(target)
		if err_create != nil {
			return &ExtractError{"Failed to create file to '" + target + ": " + err_create.Error()}
		}
		if _, err := io.Copy(out_file, tar_reader); err != nil {
			return &ExtractError{"Failed to extract file '" + target + "': " + err.Error()}
		}
		if err := out_file.Close(); err != nil {
			return &ExtractError{"Unable to finish extraction of file '" + target + "': " + err.Error()}
		}
	default:
		return &ExtractError{"Unsupported file type for '" + target + "': " + string(int(header.Typeflag))}
	}
	err_chtimes := os.Chtimes(target, header.ModTime, header.ModTime)
	if err_chtimes != nil {
		return &ExtractError{"Unable to change modification time of '" + target + "': " + err_chtimes.Error()}
	}
	return nil
}

func extract_tarball(directory string, gzip_stream io.Reader) error {
	uncompressed_stream, err := gzip.NewReader(gzip_stream)
	if err != nil {
		return err
	}

	tar_reader := tar.NewReader(uncompressed_stream)
	for true {
		header, err := tar_reader.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if strings.HasPrefix(header.Name, "/") {
			return &ExtractError{"Detected unsafe absolute path in archive: " + header.Name}
		} else if strings.Contains(header.Name, "../") {
			// This catches directory traversal traps:
			return &ExtractError{"Detected unsafe directory path: " + header.Name}
		} else if strings.Contains(header.Name, "//") {
			// Just in case forbid double slashes:
			return &ExtractError{"Detected unsafe path leading to potential absolute path in archive: " + header.Name}
		}

		target := filepath.Join(directory, header.Name)
		err_extract := extract_tar_entry(target, tar_reader, header)
		if err_extract != nil {
			return err_extract
		}
	}
	return nil
}

type OrderedYear struct {
	Path        string
	SectionKeys []string
}

func all_items_are_directories(root string, items []string) bool {
	for _, item := range items {
		path := filepath.Join(root, item)
		info, err_stat := os.Stat(path)
		if err_stat != nil {
			return false
		}
		if !info.IsDir() {
			return false
		}
	}

	return true
}

func read_meta_bytes(directory string) ([]byte, error) {
	meta_path := filepath.Join(directory, "meta.json")
	stats, err_stat := os.Stat(meta_path)
	if err_stat != nil {
		return nil, err_stat
	}
	if stats.Size() > MAX_METADATA_SIZE {
		return nil, &InputError{
			"meta.json size of " +
				strconv.FormatInt(stats.Size(), 10) +
				" bytes exceeds the maximum of " +
				strconv.FormatInt(MAX_METADATA_SIZE, 10) +
				" bytes!"}
	}
	if data, err_read := ioutil.ReadFile(meta_path); err_read != nil {
		return nil, err_read
	} else {
		return data, nil
	}
}

func read_year_info(directory string, url_path string) (OrderedYear, error) {
	data, err_data := read_meta_bytes(directory)
	empty_year := OrderedYear{}
	if err_data != nil {
		return empty_year, err_data
	}

	type YearMeta struct {
		Sections []string
	}
	var year_meta YearMeta
	err_unmarshal := json.Unmarshal(data, &year_meta)
	if err_unmarshal != nil {
		return empty_year, err_unmarshal
	}
	if !all_items_are_directories(directory, year_meta.Sections) {
		return empty_year, &InputError{"Not all sections for year '" + url_path + "' are valid!"}
	}

	return OrderedYear{
		Path:        url_path,
		SectionKeys: year_meta.Sections,
	}, nil
}

type OrderedSection struct {
	Path        string
	Name        string
	Description string
	EntryKeys   []string
}

func read_section_info(directory string, url_path string) (OrderedSection, error) {
	section := OrderedSection{}
	section.Path = url_path
	data, err_data := read_meta_bytes(directory)
	if err_data != nil {
		return section, err_data
	}

	type SectionData struct {
		Name        string
		Description string `json:""`
		Entries     []string
	}
	var meta_section SectionData
	err_unmarshal := json.Unmarshal(data, &meta_section)
	if err_unmarshal != nil {
		return section, err_unmarshal
	}
	section.Name = meta_section.Name
	section.Description = meta_section.Description
	section.EntryKeys = meta_section.Entries
	return section, nil
}

func read_entry_info(directory string, url_path string) (base.Entry, error) {
	key := filepath.Base(directory)
	entry := base.Entry{Path: url_path, Key: key}

	data, err_read := read_meta_bytes(directory)
	if err_read != nil {
		return entry, err_read
	}

	type EntryMeta struct {
		Title          string
		Author         string
		AssetType      string
		Description    string
		External_links []base.ExternalLinksSection
		Thumbnails     map[string]string
	}
	var meta_entry EntryMeta
	err_unmarshal := json.Unmarshal(data, &meta_entry)
	if err_unmarshal != nil {
		return entry, err_unmarshal
	}

	entry.Title = meta_entry.Title
	entry.Author = meta_entry.Author
	entry.AssetType = meta_entry.AssetType
	entry.Description = meta_entry.Description
	fmt.Println(meta_entry)

	_json_to_thumbnail := func(value map[string]string) (base.ImageInfo, error) {
		checksum, err := base.CreateFileChecksum(filepath.Join(directory, value["path"]))
		if err != nil {
			return base.ImageInfo{}, err
		}
		resolution_str, ok := value["resolution"]
		if !ok {
			return base.ImageInfo{}, &InputError{
				"No resolution specified for thumbnail!"}
		}
		resolution, err_res := string_to_resolution(resolution_str)
		if err_res != nil {
			return base.ImageInfo{}, err_res
		}
		return base.ImageInfo{
			url_path + "/" + value["path"],
			checksum,
			resolution,
			value["type"]}, nil
	}
	fmt.Println(meta_entry.Thumbnails)
	var err_thumbnails error
	entry.Thumbnails.Default, err_thumbnails = _json_to_thumbnail(meta_entry.Thumbnails)
	if err_thumbnails != nil {
		return entry, err_thumbnails
	}

	return entry, nil
}

func replace_path(target string, new string, old string) error {
	_, err_stat := os.Stat(target)
	if err_stat == nil {
		err_rename := os.Rename(target, old)
		if err_rename != nil {
			return err_rename
		}
	}
	err_rename := os.Rename(new, target)
	if err_rename != nil {
		return err_rename
	}
	return nil
}

func handle_year(
	settings base.SiteSettings,
	year int,
	w http.ResponseWriter,
	r *http.Request) {
	fmt.Println("Year", year)
	tmpdir, err := ioutil.TempDir(settings.DataDir, ".new-year-")
	if err != nil {
		_ise(w, err)
		return
	}
	defer os.RemoveAll(tmpdir)

	new_dir := filepath.Join(tmpdir, "new")

	err_extract := extract_tarball(new_dir, r.Body)
	if err_extract != nil {
		bad_request(w, "Invalid tar file: "+err_extract.Error())
		return
	}
	url_path := strconv.Itoa(year)
	if err := validate_year_dir(new_dir, url_path); err != nil {
		bad_request(w, "Invalid year data: "+err.Error())
	}

	target_dir := filepath.Join(settings.DataDir, strconv.Itoa(year))
	old_dir := filepath.Join(tmpdir, "old")
	err_replace := replace_path(target_dir, new_dir, old_dir)
	if err_replace != nil {
		_ise(w, err_replace)
		return
	}
	w.Write([]byte("OK\n"))
}

func validate_year_dir(dir string, url_path string) error {
	year_info, err_year := read_year_info(dir, url_path)
	if err_year != nil {
		return err_year
	}
	fmt.Println(year_info.SectionKeys)
	for _, section := range year_info.SectionKeys {
		section_dir := filepath.Join(dir, section)
		section_path := url_path + "/" + section
		if err := validate_section_dir(section_dir, section_path); err != nil {
			return err
		}
	}
	return nil
}

func directory_subdirs_match_known(dir string, known []string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	keys := make(map[string]bool)
	for _, key := range known {
		keys[key] = true
	}
	for _, fileinfo := range files {
		if !fileinfo.IsDir() {
			continue
		}
		if _, ok := keys[fileinfo.Name()]; !ok {
			return &InputError{
				"Directory '" +
					fileinfo.Name() +
					"' is not part of known directories list of " +
					strconv.Itoa(len(known)) +
					"items : " +
					strings.Join(known, ", ")}
		}
	}
	return nil
}

func validate_section_dir(dir string, url_path string) error {
	section, err := read_section_info(dir, url_path)
	if err != nil {
		return err
	}

	if err := directory_subdirs_match_known(
		dir, section.EntryKeys); err != nil {
		return err
	}
	for _, entry_key := range section.EntryKeys {
		entry_dir := filepath.Join(dir, entry_key)
		entry_path := url_path + "/" + entry_key
		if err := validate_entry_dir(entry_dir, entry_path); err != nil {
			return err
		}
	}
	return nil
}

func validate_entry_dir(dir string, url_path string) error {
	_, err := read_entry_info(dir, url_path)
	return err
}

func handle_section(
	settings base.SiteSettings,
	year int,
	section string,
	w http.ResponseWriter,
	r *http.Request) {
	url_path := strconv.Itoa(year) + "/" + section
	yeardir := path.Join(settings.DataDir, strconv.Itoa(year))
	target_dir := path.Join(yeardir, section)
	if _, err := os.Stat(target_dir); err != nil {
		bad_request(w, "Can only update an existing section")
		return
	}
	var tmpdir string
	{
		_tmpdir, err := ioutil.TempDir(yeardir, ".new-section-")
		if err != nil {
			_ise(w, err)
			return
		}
		tmpdir = _tmpdir
	}
	defer os.RemoveAll(tmpdir)

	new_dir := filepath.Join(tmpdir, "new")
	err_extract := extract_tarball(new_dir, r.Body)
	if err_extract != nil {
		bad_request(w, "Invalid tar file: "+err_extract.Error())
		return
	}
	if err := validate_section_dir(new_dir, url_path); err != nil {
		bad_request(w, "Invalid section data: "+err.Error())
	}

	old_dir := filepath.Join(tmpdir, "old")
	err_replace := replace_path(target_dir, new_dir, old_dir)
	if err_replace != nil {
		_ise(w, err_replace)
		return
	}
	w.Write([]byte("OK\n"))
}

func renderer(settings base.SiteSettings, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		// Only accept PUT method.
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Method Not Allowed.\n"))
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) > 2 {
		bad_request(w, "Can only update either a year or a section!\n")
		return
	}
	year_str := parts[0]
	matched_year, err_year := regexp.MatchString("^\\d{4}$", year_str)
	if err_year != nil {
		_ise(w, err_year)
	}
	if !matched_year {
		bad_request(w, "Year '"+year_str+"' is not a number!")
		return
	}
	year, _ := strconv.Atoi(year_str)
	if len(parts) == 1 {
		handle_year(settings, year, w, r)
		return
	}
	section := parts[1]
	matched_section, err_section := regexp.MatchString("^[a-z]([a-z]+-)*[a-z]+$", section)
	if err_section != nil {
		_ise(w, err_section)
		return
	}
	if !matched_section {
		bad_request(w, "Illegal section name '"+section+"'!")
		return
	}
	handle_section(settings, year, section, w, r)
}

func Renderer(settings base.SiteSettings, state *state.SiteState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderer(settings, w, r)
	}
}
