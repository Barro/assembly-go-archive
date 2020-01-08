package server

import (
	"bufio"
	"crypto/subtle"
	"fileperm"
	"log"
	"net/http"
	"os"
	"strings"
)

type UsernamePasswordError struct {
	message string
}

func (error *UsernamePasswordError) Error() string {
	return error.message
}

type AuthData map[string]string

func read_auth_data(filename string) (AuthData, error) {
	if fileperm.IsFileWideOpen(filename) {
		return nil, &UsernamePasswordError{"File " + filename + " should only be readable by the current user!"}
	}
	m := make(map[string]string)
	file, err := os.Open(filename)
	if err != nil {
		log.Printf(
			"Unable to read authentication data from %s: %s", filename, err)
		return m, nil
	}

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

func Ise(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal server error!\n"))
}

func BasicAuth(auth_filename string, handler http.HandlerFunc) http.HandlerFunc {
	if fileperm.IsFileWideOpen(auth_filename) {
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
			Ise(w)
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

func StripPrefix(prefix string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = r.URL.Path[len(prefix):]
		handler(w, r)
	}
}

func AddCacheHeadersFunc(w http.ResponseWriter, r *http.Request) {
	// Only add cache headers when it's likely that the query
	// string includes the checksum (36 bits of base64 encoded
	// data) of the requested resource.
	if len(r.URL.RawQuery) >= 6 {
		header := w.Header()
		header.Add(
			"Cache-Control", "immutable, public, max-age=2592000")
	}
}

func AddCacheHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AddCacheHeadersFunc(w, r)
		h.ServeHTTP(w, r)
	})
}
