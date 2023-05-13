package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	s := fmt.Sprintf("internal server error: %v\n", err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(s))

	log.Println(s)
}

func notFound(w http.ResponseWriter, r *http.Request, v interface{}) {
	s := fmt.Sprintf("not found: %v\n", v)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(s))

	log.Println(s)
}

func notSupported(w http.ResponseWriter, r *http.Request, v interface{}) {
	s := fmt.Sprintf("not supported: %q %v\n", r.Method, v)
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(s))

	log.Println(s)
}

func jsonResponse(w http.ResponseWriter, r *http.Request, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)

	log.Printf("%v", string(b))
}
