package controller

import (
	http "net/http"
	strings "strings"
)

type FileSystem struct {
	fs http.FileSystem
}

func NewFilesystem(directory string) *FileSystem {
	return &FileSystem{http.Dir(directory)}
}

func (fs FileSystem) Open(path string) (http.File, error) {
	f, err := fs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := fs.fs.Open(index); err != nil {
			return nil, err
		}
	}

	return f, nil
}
