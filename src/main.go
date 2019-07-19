package main

import (
	"api"
	"base"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"server"
	"site"
	"state"
)

func RenderTeapot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte("I'm a teapot\n"))
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
		DataDir:      *data_dir,
		StaticDir:    *static_dir,
		TemplatesDir: *templates_dir,
	}
	state := &state.SiteState{}
	if *devmode {
		log.Println("Development mode enabled. DO NOT USE THIS IN PUBLIC! /exit is enabled!")
		http.HandleFunc("/exit/", exit)
	} else {
		http.HandleFunc("/exit/", exit_forbidden)
	}
	http.HandleFunc("/api/", server.StripPrefix("/api/",
		server.BasicAuth(*authfile, api.Renderer(settings, state))))
	http.HandleFunc("/site/", server.StripPrefix("/site/",
		site.SiteRenderer(settings, state)))
	http.HandleFunc("/teapot/", RenderTeapot)
	http.Handle("/site/_data/", http.StripPrefix("/site/_data/", http.FileServer(http.Dir(settings.DataDir))))
	http.Handle("/site/_static/", http.StripPrefix("/site/_static/", http.FileServer(http.Dir(settings.StaticDir))))
	http.HandleFunc("/", RenderLinks)
	listen_addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Listening to %s", listen_addr)
	log.Fatal(http.ListenAndServe(listen_addr, nil))
}
