package server

import (
	http "net/http"
)

type staticFile struct {
	path string
}

func NewStaticFile(path string) func(rw http.ResponseWriter, req *http.Request) {
	s := &staticFile{
		path: path,
	}
	return s.handle
}

func (s *staticFile) handle(rw http.ResponseWriter, req *http.Request) {
	http.ServeFile(rw, req, s.path)
}
