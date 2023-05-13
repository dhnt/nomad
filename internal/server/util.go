package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// resolveArgs prepends root to the arg if it is prefixed with file: scheme
// or preceded by an arg specified in the options.
func resolveArgs(root string, options []string, args []string) ([]string, error) {
	var opt string
	resolve := func(arg string) (string, error) {
		if strings.HasPrefix(arg, "file:") {
			u, err := url.Parse(arg)
			if err != nil {
				return "", err
			}
			return filepath.Join(root, u.Path), nil
		}
		for _, v := range options {
			if v == opt {
				return filepath.Join(root, arg), nil
			}
		}
		return arg, nil
	}

	resolved := []string{}

	for _, v := range args {
		// including --
		if strings.HasPrefix(v, "-") {
			opt = v
			resolved = append(resolved, v)
			continue
		}
		arg, err := resolve(v)
		if err != nil {
			return nil, err
		}
		opt = ""
		resolved = append(resolved, arg)
	}

	return resolved, nil
}

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

func statusOK(w http.ResponseWriter, r *http.Request, v interface{}) {
	s := fmt.Sprintf("OK: %q %q %v\n", r.Method, r.URL.Path, v)
	w.WriteHeader(http.StatusOK)
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
