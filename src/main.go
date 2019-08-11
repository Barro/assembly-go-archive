package main

import (
	"api"
	"base"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"server"
	"site"
	"state"
	"strings"
	"sync"
)

func RenderTeapot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte("I'm a teapot\n"))
}

func RenderFaviconFunc(static_dir string) http.HandlerFunc {
	favicon_path := filepath.Join(static_dir, "favicon.ico")
	favicon_data, err := ioutil.ReadFile(favicon_path)
	if err != nil {
		panic(fmt.Sprintf(
			"Unable to read favicon from %s: %v", favicon_path, err))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		server.AddCacheHeadersFunc(w, r)
		w.Write(favicon_data)
	}
}

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(ioutil.Discard)
		return w
	},
}

type GzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *GzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *GzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func CompressGzipHandler(
	match_paths *regexp.Regexp,
	next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if !match_paths.MatchString(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")

		gz := gzPool.Get().(*gzip.Writer)
		defer gzPool.Put(gz)

		gz.Reset(w)
		defer gz.Close()
		gzip_writer := GzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
		}
		next.ServeHTTP(&gzip_writer, r)
	})
}

func exit_forbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte("Can not exit without -dev mode!\n"))
}

func RenderLinks(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html>
<head><title>Namespaces</title></head>
<body>
<p>This offers following namespaces:</p>
<ul>
<li><a href="/api/">/api/</a> for database manipulation. Requires authentication.</li>
<li><a href="/site/">/site/</a> should be exposed through a reverse proxy as the site root</li>
<li><a href="/teapot/">/teapot/</a> I'm a teapot!</li>
<li><a href="/exit/">/exit/</a> make me quit, only in <code>-dev</code> mode</li>
</ul>
</body>
</html>
`))
}

// Terminate by client request.
func exit(w http.ResponseWriter, r *http.Request) {
	user, _, _ := r.BasicAuth()
	ip_address := r.RemoteAddr
	forwarded_for := r.Header.Get("X-Forwarded-For")

	log.Println(
		"Exit request from '" + user + "' at " + ip_address + " <" + forwarded_for + ">")

	os.Exit(0)
}

func main() {
	host := flag.String("host", "localhost", "Host interface to listen to")
	port := flag.Int("port", 8080, "Port to listen to")
	data_dir := flag.String("dir-data", "_data", "Data directory")
	static_dir := flag.String("dir-static", "_static", "Static files directory")
	templates_dir := flag.String(
		"dir-templates", "templates", "Site templates directory")
	authfile := flag.String("authfile", "auth.txt", "File with username:password lines")
	devmode := flag.Bool("dev", false, "Enable development mode")

	flag.Parse()

	settings := base.SiteSettings{
		SiteRoot:     "",
		DataDir:      *data_dir,
		StaticDir:    *static_dir,
		TemplatesDir: *templates_dir,
	}

	if *devmode {
		log.Println("Development mode enabled. DO NOT USE THIS IN PUBLIC! /exit is enabled!")
		settings.SiteRoot = "/site"
		http.HandleFunc("/exit/", exit)
	} else {
		http.HandleFunc("/exit/", exit_forbidden)
	}

	state, err_state := state.New(settings.DataDir, settings.SiteRoot)
	if err_state != nil {
		log.Fatal(err_state)
	}

	http.HandleFunc("/api/", server.StripPrefix("/api/",
		server.BasicAuth(*authfile, api.Renderer(settings, state))))

	http.Handle("/site/",
		CompressGzipHandler(
			regexp.MustCompile(""),
			server.StripPrefix("/site/",
				http.HandlerFunc(site.SiteRenderer(settings, state)))))
	http.HandleFunc("/teapot/", RenderTeapot)
	http.Handle(
		"/site/_data/",
		server.AddCacheHeaders(
			CompressGzipHandler(
				regexp.MustCompile("(ico|js|css|json)$"),
				http.StripPrefix(
					"/site/_data/",
					http.FileServer(http.Dir(settings.DataDir))))))
	http.Handle(
		"/site/_static/",
		server.AddCacheHeaders(
			CompressGzipHandler(
				regexp.MustCompile("(ico|js|css|json)$"),
				http.StripPrefix(
					"/site/_static/",
					http.FileServer(http.Dir(settings.StaticDir))))))
	http.Handle(
		"/site/favicon.ico",
		CompressGzipHandler(
			regexp.MustCompile(""),
			http.HandlerFunc(RenderFaviconFunc(settings.StaticDir))))
	http.HandleFunc("/", RenderLinks)
	listen_addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Listening to %s", listen_addr)
	log.Fatal(http.ListenAndServe(listen_addr, nil))
}
