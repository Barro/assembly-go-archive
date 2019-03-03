package import_test

import (
	"api"
	"archive/tar"
	"base"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server"
	"strconv"
	"testing"
)

func create_site_layout(t *testing.T) *base.SiteSettings {
	data_dir := filepath.Join(t.Name(), "data")
	{
		err := os.MkdirAll(data_dir, 0700)
		if err != nil {
			t.Fatal(err)
		}
	}
	unreadable_dir := filepath.Join(t.Name(), "unreadable")
	{
		err := os.MkdirAll(unreadable_dir, 0000)
		if err != nil {
			t.Fatal(err)
		}
	}

	settings := base.SiteSettings{
		DataDir:      data_dir,
		StaticDir:    unreadable_dir,
		TemplatesDir: unreadable_dir,
	}
	return &settings
}

func do_request(t *testing.T, path string, body io.Reader) (*base.SiteSettings, *http.Response) {
	settings := create_site_layout(t)
	renderer := api.Renderer(*settings)
	handler := server.StripPrefix("/api/", renderer)
	url := "http://example.com/api/" + path
	req := httptest.NewRequest("PUT", url, body)
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	return settings, resp
}

func require_http_success(t *testing.T, resp *http.Response) {
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Error("Unsuccessful status code: " + strconv.Itoa(resp.StatusCode) + "\n" + string(body))
		t.FailNow()
	}
}

func list_files(t *testing.T, settings *base.SiteSettings) {
	err := filepath.Walk(settings.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Errorf("Unable to read path %s: %s", path, err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err_rel := filepath.Rel(settings.DataDir, path)
		if err_rel != nil {
			t.Error(path)
		} else {
			t.Error(relative)
		}
		return nil
	})
	if err != nil {
		t.Errorf("Unable to walk data directory %s: %s", settings.DataDir, err)
	}
}

func require_files(t *testing.T, settings *base.SiteSettings, files []string) {
	for _, filename := range files {
		_, err := os.Stat(filepath.Join(settings.DataDir, filename))
		if err != nil {
			t.Errorf("Missing file %s", filename)
			t.Error("Have following files:")
			list_files(t, settings)
			t.FailNow()
		}
	}
}

type TarEntry struct {
	Path string
	Data string
}

func create_tarball(t *testing.T, files []TarEntry) io.Reader {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Path,
			Mode: 0600,
			Size: int64(len(file.Data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.Data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var gz_buf bytes.Buffer
	gw := gzip.NewWriter(&gz_buf)
	if _, err := gw.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	return bytes.NewReader(gz_buf.Bytes())
}

func TestImport(t *testing.T) {
	section_data := create_tarball(t, []TarEntry{
		{"meta.json", "{}"},
		{"entry/meta.json", `{
"title": "Title",
"author": "Author",
"asset": {}
}`},
	})
	settings, resp := do_request(t, "2001/section", section_data)
	require_http_success(t, resp)
	require_files(t, settings, []string{
		"2001/section/meta.json",
		"2001/section/entry/meta.json",
	})
}
