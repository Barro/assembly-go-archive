package api

import (
	"archive/tar"
	"base"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
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
	"strconv"
	"strings"
)

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
		err_mkdir := os.MkdirAll(parent_directory, 0755)
		if err_mkdir != nil {
			return &ExtractError{"Failed to create parent directory for '" + target + "': " + err_mkdir.Error()}
		}

		out_file, err_create := os.Create(target)
		if err_create != nil {
			return &ExtractError{"Failed to create file to '" + target + ": " + err_create.Error()}
		}
		_, err_copy := io.Copy(out_file, tar_reader)
		if err_copy != nil {
			return &ExtractError{"Failed to extract file '" + target + "': " + err_copy.Error()}
		}
		out_file.Close()
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
		sections []string
	}
	var year_meta YearMeta
	err_unmarshal := json.Unmarshal(data, &year_meta)
	if err_unmarshal != nil {
		return empty_year, err_unmarshal
	}
	if !all_items_are_directories(directory, year_meta.sections) {
		return empty_year, &InputError{"Not all sections for year '" + url_path + "' are valid!"}
	}

	return OrderedYear{
		Path:        url_path,
		SectionKeys: year_meta.sections,
	}, nil
}

type OrderedSection struct {
	Path        string
	Description string
	EntryKeys   []string
}

func read_section_info(directory string, url_path string) (OrderedSection, error) {
	section := OrderedSection{}
	data, err_data := read_meta_bytes(directory)
	if err_data != nil {
		return section, err_data
	}

	type SectionData struct {
		name        string
		description string
		entries     []string
	}
	var meta_section SectionData
	err_unmarshal := json.Unmarshal(data, &meta_section)
	if err_unmarshal != nil {
		return section, err_unmarshal
	}
	return section, nil
}

func read_entry_info(directory string, url_path string) (base.EntryInfo, error) {
	key := filepath.Base(directory)
	entry := base.EntryInfo{Path: url_path, Key: key}

	data, err_read := read_meta_bytes(directory)
	if err_read != nil {
		return entry, err_read
	}
	var meta_json_raw interface{}
	err_unmarshal := json.Unmarshal(data, &meta_json_raw)
	if err_unmarshal != nil {
		return entry, err_unmarshal
	}

	meta_root := meta_json_raw.(map[string]interface{})
	entry.Title = meta_root["title"].(string)
	entry.Author = meta_root["author"].(string)
	entry.Asset = meta_root["asset"].(string)

	_json_to_thumbnail := func(value map[string]string) (base.ThumbnailInfo, error) {
		checksum, err := create_file_checksum(filepath.Join(directory, value["path"]))
		if err != nil {
			return base.ThumbnailInfo{}, err
		}
		resolution, err_res := string_to_resolution(value["resolution"])
		if err_res != nil {
			return base.ThumbnailInfo{}, err_res
		}
		return base.ThumbnailInfo{
			url_path + "/" + value["path"],
			&checksum,
			resolution,
			value["type"]}, nil
	}
	var err_thumbnails error
	entry.Thumbnails.Default, err_thumbnails = _json_to_thumbnail(
		meta_root["thumbnail"].(map[string]string))
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
	url_path := string(year)
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
	for _, section := range year_info.SectionKeys {
		section_dir := filepath.Join(dir, section)
		section_path := url_path + "/" + section
		if err := validate_section_dir(section_dir, section_path); err != nil {
			return err
		}
	}
	return nil
}

func validate_section_dir(dir string, url_path string) error {
	_, err := read_section_info(dir, url_path)
	if err != nil {
		return err
	}
	return nil
}

func handle_section(
	settings base.SiteSettings,
	year int,
	section string,
	w http.ResponseWriter,
	r *http.Request) {
	url_path := string(year) + "/" + section
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

func Renderer(settings base.SiteSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderer(settings, w, r)
	}
}
