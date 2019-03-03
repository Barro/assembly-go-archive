package import_test

import (
	"api"
	"base"
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

func require_files(t *testing.T, settings *base.SiteSettings, files []string) {
	for _, filename := range files {
		_, err := os.Stat(filepath.Join(settings.DataDir, filename))
		if err != nil {
			t.Fatalf("Missing file: %s", err)
		}
	}
}

type TarFile struct {
	path string
	data string
}

func create_tarball(t *testing.T, files []TarFile) io.Reader {
	return nil
}

func TestImport(t *testing.T) {
	section_data := create_tarball(t, []TarFile{
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
