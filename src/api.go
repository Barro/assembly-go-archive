package api

import (
	"fmt"
	"net/http"
)

func handle_year(w http.ResponseWriter, r *http.Request) {

}

func handle_section(w http.ResponseWriter, r *http.Request) {

}

func Render(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Method Not Allowed.\n"))
		return
	}
	fmt.Println(r.URL)
}
