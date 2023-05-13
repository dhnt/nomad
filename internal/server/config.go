package server

import (
	"net/url"
)

type ServerConfig struct {
	Port int

	Root string
	Url  *url.URL
}
