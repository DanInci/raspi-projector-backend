package server

import (
	fmt "fmt"
	io "io"
	http "net/http"
	path "path"
	strings "strings"
)

const ASSETS_FOLDER = "assets"

type FileHandler struct {
	dir http.Dir
	fs  http.Handler
}

func NewStaticServer(folder string) *FileHandler {
	dir := http.Dir(folder)
	s := &FileHandler{
		dir: dir,
		fs:  http.FileServer(dir),
	}
	return s
}

func (fh *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path

	for {
		dirname := cut(upath)
		if strings.HasSuffix(dirname, fmt.Sprintf("%s/", ASSETS_FOLDER)) {
			break
		} else {
			upath = path.Join(dirname, "index.html")
			indexHTML, err := fh.dir.Open(upath)
			if err == nil {
				indexHTML.Close()
				break
			} else if dirname == "" {
				return
			}
		}
	}

	file, err := fh.dir.Open(upath)
	defer file.Close()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Something went wrong - " + http.StatusText(http.StatusNotFound)))
	} else {
		if strings.HasSuffix(upath, ".js") {
			w.Header().Set("Content-Type", "text/javascript")
		} else if strings.HasSuffix(upath, ".css") {
			w.Header().Set("Content-Type", "text/css")
		}
		buffer := make([]byte, 1024)
		for {
			n, err := file.Read(buffer)
			if err == io.EOF {
				break
			} else {
				w.Write(buffer[:n])
			}
		}
	}
}

func cut(name string) string {
	name = strings.TrimSuffix(name, "/")
	dir, _ := path.Split(name)
	return dir
}
