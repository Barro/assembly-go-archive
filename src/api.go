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
	"path/filepath"
	"regexp"
	"strings"
)

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
		}

		target := filepath.Join(directory, header.Name)
		err_extract := extract_tar_entry(target, tar_reader, header)
		if err_extract != nil {
			return err_extract
		}
	}
	return nil
}

func handle_year(
	settings base.SiteSettings,
	year string,
	w http.ResponseWriter,
	r *http.Request) {
	fmt.Println("Year", year)
	tmpdir, err := ioutil.TempDir(settings.DataDir, ".new-year-")
	if err != nil {
		_ise(w, err)
		return
	}
	defer os.RemoveAll(tmpdir)

	// Make the temporary directory world readable. It doesn't matter
	// if someone could theoretically read this, as it will be public
	// anyways.
	os.Chmod(tmpdir, 0755)

	new_dir := filepath.Join(tmpdir, "new")

	err_extract := extract_tarball(new_dir, r.Body)
	if err_extract != nil {
		bad_request(w, "Invalid tar file: "+err_extract.Error())
		return
	}
	w.Write([]byte("OK\n"))
}

func handle_section(
	settings base.SiteSettings,
	year string,
	section string,
	w http.ResponseWriter,
	r *http.Request) {
	fmt.Println("Section", year, section)
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
	year := parts[0]
	matched_year, err_year := regexp.MatchString("^\\d{4}$", year)
	if err_year != nil {
		_ise(w, err_year)
	}
	if !matched_year {
		bad_request(w, "Year '"+year+"' is not a number!")
		return
	}
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
