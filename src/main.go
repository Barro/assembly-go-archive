package main

import (
	"api"
	"base"
	"bufio"
	"crypto/subtle"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"site"
	"strings"
)

type UsernamePasswordError struct {
	message string
}

func (error *UsernamePasswordError) Error() string {
	return error.message
}

func is_file_wide_open(filename string) bool {
	stats, err := os.Stat(filename)
	if err != nil {
		return false
	}
	if stats.Mode()&044 != 0 {
		return true
	}
	return false
}

type AuthData map[string]string

func read_auth_data(filename string) (AuthData, error) {
	if is_file_wide_open(filename) {
		return nil, &UsernamePasswordError{"File " + filename + " should only be readable by the current user!"}
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		up_raw := strings.SplitN(line, ":", 2)
		if up_raw == nil {
			return nil, &UsernamePasswordError{"Line " + line + " does not include colon!"}
		}
		m[up_raw[0]] = up_raw[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return m, nil
}

func has_username_password(users AuthData, username, password string) bool {
	// This line here opens up the possibility of username
	// enumeration:
	user_password, key_ok := users[username]
	if !key_ok {
		return false
	}

	if subtle.ConstantTimeCompare([]byte(password), []byte(user_password)) != 1 {
		return false
	}
	return true
}

func _ise(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error!\n"))
}

func BasicAuth(auth_filename string, handler http.HandlerFunc) http.HandlerFunc {
	if is_file_wide_open(auth_filename) {
		log.Fatal("File " + auth_filename + " should only be readable by the current user!")
	}

	_unauthorized := func(w http.ResponseWriter) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Assembly Archive API"`)
		w.WriteHeader(401)
		w.Write([]byte("Unauthorised.\n"))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			_unauthorized(w)
			return
		}
		users, err := read_auth_data(auth_filename)
		if err != nil {
			_ise(w)
			log.Print(err)
			return
		}
		if !has_username_password(users, user, pass) {
			_unauthorized(w)
			return
		}

		handler(w, r)
	}
}

func RenderTeapot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	w.Write([]byte("I'm a teapot\n"))
}

func StripPrefix(prefix string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = r.URL.Path[len(prefix):]
		handler(w, r)
	}
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
	if *devmode {
		log.Println("Development mode enabled. DO NOT USE THIS IN PUBLIC! /exit is enabled!")
		http.HandleFunc("/exit", exit)
	}
	http.HandleFunc("/api/", StripPrefix("/api/", BasicAuth(*authfile, api.Renderer(settings))))
	http.HandleFunc("/site/", StripPrefix("/site/", site.SiteRenderer(settings)))
	http.HandleFunc("/teapot/", RenderTeapot)
	http.Handle("/site/_data/", http.StripPrefix("/site/_data/", http.FileServer(http.Dir(settings.DataDir))))
	http.Handle("/site/_static/", http.StripPrefix("/site/_static/", http.FileServer(http.Dir(settings.StaticDir))))
	listen_addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Listening to %s", listen_addr)
	log.Fatal(http.ListenAndServe(listen_addr, nil))
}
