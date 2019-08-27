package api

import (
	"archive/tar"
	"base"
	"compress/gzip"
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

func _ise(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error!\n"))
	log.Panic(err)
}

func bad_request(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message + "\n"))
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
	site_state *state.SiteState,
	year int,
	w http.ResponseWriter,
	r *http.Request) {
	url_path := fmt.Sprintf("%s/%d", settings.SiteRoot, year)
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

	year_data, err_read := state.ReadYear(
		new_dir,
		fmt.Sprintf("%s/_data/%d", settings.SiteRoot, year),
		url_path,
		strconv.Itoa(year))
	if err_read != nil {
		bad_request(w, "Invalid year data: "+err_read.Error())
		return
	}
	if year_data == nil {
		bad_request(
			w, fmt.Sprintf("Year data for year %d is out of range", year))
		return
	}

	target_dir := filepath.Join(settings.DataDir, strconv.Itoa(year))
	old_dir := filepath.Join(tmpdir, "old")
	err_replace := replace_path(target_dir, new_dir, old_dir)
	if err_replace != nil {
		_ise(w, err_replace)
		return
	}
	year_added := false
	var mod_years []*base.Year
	var next_year *base.Year
	for _, prev_year := range site_state.Years {
		if prev_year.Year == year {
			mod_years = append(mod_years, year_data)
			next_year = year_data
			year_added = true
			continue
		}
		if next_year != nil && prev_year.Year < year && year < next_year.Year {
			mod_years = append(mod_years, year_data)
			year_added = true
		}
		next_year = prev_year
		mod_years = append(mod_years, prev_year)
	}
	if !year_added {
		if next_year == nil {
			mod_years = []*base.Year{year_data}
		} else if year < next_year.Year {
			mod_years = append(mod_years, year_data)
		} else {
			mod_years = append([]*base.Year{year_data}, mod_years...)
		}
	}
	site_state.Years = mod_years
	w.Write([]byte("OK\n"))
}

func handle_section(
	settings base.SiteSettings,
	site_state *state.SiteState,
	year *base.Year,
	key string,
	w http.ResponseWriter,
	r *http.Request) {
	url_path := fmt.Sprintf("%s/%s/%s", settings.SiteRoot, year.Key, key)
	yeardir := path.Join(settings.DataDir, year.Key)
	target_dir := path.Join(yeardir, key)
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
	section_data, err_section := state.ReadSection(
		new_dir,
		fmt.Sprintf("%s/_data/%s/%s", settings.SiteRoot, year.Key, key),
		url_path,
		key)
	if err_section != nil {
		bad_request(w, "Invalid section data: "+err_section.Error())
		return
	}

	old_dir := filepath.Join(tmpdir, "old")
	err_replace := replace_path(target_dir, new_dir, old_dir)
	if err_replace != nil {
		_ise(w, err_replace)
		return
	}
	for i, old_section := range year.Sections {
		if old_section.Key == key {
			year.Sections[i] = section_data
			break
		}
	}
	w.Write([]byte("OK\n"))
}

func renderer(
	settings base.SiteSettings,
	state *state.SiteState,
	w http.ResponseWriter,
	r *http.Request) {
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
	year_int, _ := strconv.Atoi(year_str)
	if len(parts) == 1 {
		handle_year(settings, state, year_int, w, r)
		return
	}
	var year *base.Year
	for _, year_candidate := range state.Years {
		if year_candidate.Key == year_str {
			year = year_candidate
			break
		}
	}
	if year == nil {
		bad_request(w, "No previous year defined for section!")
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
	handle_section(settings, state, year, section, w, r)
}

func Renderer(settings base.SiteSettings, state *state.SiteState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderer(settings, state, w, r)
	}
}
