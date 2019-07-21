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
	"strconv"
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

func create_sections(site_root string, year base.Year) []*base.Section {
	entry := base.Entry{
		Path:          "/2018/section/entry",
		Key:           "entry",
		Title:         "title",
		Author:        "author",
		Asset:         "asset",
		Description:   "description",
		ExternalLinks: []base.ExternalLinksSection{},
		Thumbnails: base.Thumbnails{
			Default: base.ThumbnailInfo{
				Path:     site_root + "/_data/2018/music-background.jpeg",
				Checksum: nil,
				Size: base.Resolution{
					X: 160,
					Y: 90,
				},
				Type: "image/png",
			},
			Extra: []base.TypedThumbnails{},
		},
	}

	var sections []*base.Section
	section_ranked := base.Section{
		Path:        "/2018/section-ranked/",
		Key:         "section-ranked",
		Name:        "Section ranked",
		Description: "Here is a decent ranked description!",
		IsRanked:    true,
	}
	section_unranked := base.Section{
		Path:        "/2018/section-unranked/",
		Key:         "section-unranked",
		Name:        "Section unranked",
		Description: "Here is a decent unranked description!",
		IsRanked:    false,
	}
	for i := 0; i < 25; i++ {
		new_section_ranked := section_ranked
		new_section_ranked.Key = new_section_ranked.Key + "-" + strconv.Itoa(i)
		new_section_ranked.Name = new_section_ranked.Name + " " + strconv.Itoa(i)
		var entries_ranked []*base.Entry
		for i := 0; i < 20; i++ {
			new_entry := entry
			new_entry.Key = new_entry.Key + "-" + strconv.Itoa(i)
			new_entry.Title = new_entry.Title + "-" + strconv.Itoa(i)
			entries_ranked = append(entries_ranked, &new_entry)
		}
		new_section_ranked.Entries = entries_ranked
		sections = append(sections, &new_section_ranked)

		new_section_unranked := section_unranked
		new_section_unranked.Key = new_section_unranked.Key + "-" + strconv.Itoa(i)
		new_section_unranked.Name = new_section_unranked.Name + " " + strconv.Itoa(i)
		copy(new_section_unranked.Entries, section_unranked.Entries)
		var entries_unranked []*base.Entry
		for i := 0; i < 20; i++ {
			new_entry := entry
			new_entry.Key = new_entry.Key + "-" + strconv.Itoa(i)
			new_entry.Title = new_entry.Title + "-" + strconv.Itoa(i)
			entries_unranked = append(entries_unranked, &new_entry)
		}
		new_section_unranked.Entries = entries_unranked
		sections = append(sections, &new_section_unranked)
	}
	return sections
}

func adjust_paths(years []*base.Year) {
	for _, year := range years {
		for _, section := range year.Sections {
			section.Path = year.Path + "/" + section.Key
			new_entries := section.Entries
			section.Entries = new_entries
			for i, entry := range section.Entries {
				new_entry := *entry
				new_entry.Path = section.Path + "/" + new_entry.Key
				section.Entries[i] = &new_entry
			}
		}
	}
}

func seed_site_state(site_root string) state.SiteState {
	var years []*base.Year
	for i := 2030; i >= 1990; i-- {
		new_year := base.Year{
			Year: i,
			Path: site_root + "/" + strconv.Itoa(i),
			Key:  strconv.Itoa(i),
			Name: strconv.Itoa(i),
		}
		new_year.Sections = create_sections(site_root, new_year)
		years = append(years, &new_year)
	}
	state := state.SiteState{
		Years: years,
	}
	adjust_paths(state.Years)
	return state
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

	state := seed_site_state(settings.SiteRoot)

	http.HandleFunc("/api/", server.StripPrefix("/api/",
		server.BasicAuth(*authfile, api.Renderer(settings, &state))))
	http.HandleFunc("/site/", server.StripPrefix("/site/",
		site.SiteRenderer(settings, &state)))
	http.HandleFunc("/teapot/", RenderTeapot)
	http.Handle("/site/_data/", http.StripPrefix("/site/_data/", http.FileServer(http.Dir(settings.DataDir))))
	http.Handle("/site/_static/", http.StripPrefix("/site/_static/", http.FileServer(http.Dir(settings.StaticDir))))
	http.HandleFunc("/", RenderLinks)
	listen_addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Listening to %s", listen_addr)
	log.Fatal(http.ListenAndServe(listen_addr, nil))
}
