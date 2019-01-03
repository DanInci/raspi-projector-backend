package controller

import (
	json "encoding/json"
	impress "github.com/DanInci/raspberry-projector/impress"
	log "github.com/apsdehal/go-logger"
	websocket "github.com/gorilla/websocket"
	ioutil "io/ioutil"
	http "net/http"
	os "os"
	filepath "path/filepath"
	strings "strings"
	sync "sync"
)

var Logger *log.Logger

var impressClient *impress.ImpressClient
var mu sync.Mutex = sync.Mutex{}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const OWNER_UUID_HEADER = "X-OWNER-UUID"

func GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if isSlideShowRunning() {
		w.WriteHeader(http.StatusOK)

		stats := getImpressClient().GetStats()
		encoded, _ := json.Marshal(stats)
		w.Write(encoded)

	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func UploadPPT(w http.ResponseWriter, r *http.Request, filesDirectory string, maxUploadSize int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, "File is too big", http.StatusBadRequest)
		return
	}

	fileName := r.PostFormValue("fileName")
	if fileName == "" {
		writeError(w, "'fileName' field not found", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("uploadFile")
	if err != nil {
		writeError(w, "'uploadFile' field not found", http.StatusBadRequest)
		return
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		writeError(w, "Invalid file", http.StatusBadRequest)
		return
	}
	fileType := http.DetectContentType(fileBytes)
	if fileType != "application/octet-stream" || !(strings.HasSuffix(fileName, ".ppt") || strings.HasSuffix(fileName, ".pptx")) {
		writeError(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	if isSlideShowRunning() {
		writeError(w, "Slideshow already running", http.StatusBadRequest)
		return
	}

	uploadFolderPath := filepath.Join(filepath.Dir(os.Args[0]), filesDirectory)
	os.MkdirAll(uploadFolderPath, os.ModePerm)
	filePath := filepath.Join(uploadFolderPath, fileName)

	newFile, err := os.Create(filePath)
	if err != nil {
		Logger.ErrorF("Failed to upload file: %v", err)
		writeError(w, "Failed to write file", http.StatusInternalServerError)
		return
	}
	defer newFile.Close()
	if _, err := newFile.Write(fileBytes); err != nil {
		Logger.ErrorF("Failed to upload file: %v", err)
		writeError(w, "Failed to write file", http.StatusInternalServerError)
		return
	}
	Logger.InfoF("Uploaded file: %s\n", filePath)

	uuid := generateUUID()
	client := impress.NewClient()
	if err := client.StartPresentation(uuid, filePath); err != nil {
		Logger.ErrorF("Failed to start impress presentation: %v", err)
		client.Terminate()
		writeError(w, "Slideshow failed to start", http.StatusInternalServerError)
		return
	}
	if err := client.OpenConnection(); err != nil {
		Logger.ErrorF("Failed to open impress remote connection: %v", err)
		client.Terminate()
		writeError(w, "Slideshow failed to start", http.StatusInternalServerError)
		return
	}
	client.ListenAndServe()
	setImpressClient(client)

	toEncode := make(map[string]interface{})
	toEncode["ownerUUID"] = uuid
	encoded, _ := json.Marshal(toEncode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(encoded)

}

func ServeImpressController(w http.ResponseWriter, r *http.Request) {
	if !isSlideShowRunning() {
		writeError(w, "Slideshow not running", http.StatusBadRequest)
		return
	}

	client := getImpressClient()
	if !client.HasControllerSpace() {
		writeError(w, "Slideshow has reached the maximum number of controllers", http.StatusBadRequest)
		return
	}

	ownerHeader := r.Header.Get(OWNER_UUID_HEADER)
	isOwner := ownerHeader != "" && isSlideShowOwnerUUID(ownerHeader)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logger.WarningF("Failed to upgrade to socket connection from %s: %v", r.RemoteAddr, err)
		return
	}
	Logger.InfoF("New socket controller from %s", r.RemoteAddr)

	controller := impress.NewController(conn, isOwner)
	controller.StartPumping(client)
}

func Terminate(server *http.Server) {
	if client := getImpressClient(); client != nil {
		client.Terminate()
	}
	server.Shutdown(nil)
}

func writeError(w http.ResponseWriter, message string, status int) {
	toEncode := make(map[string]interface{})
	toEncode["error"] = message
	encoded, _ := json.Marshal(toEncode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(encoded)
}

func getImpressClient() *impress.ImpressClient {
	mu.Lock()
	defer mu.Unlock()

	return impressClient
}

func setImpressClient(impr *impress.ImpressClient) {
	mu.Lock()
	defer mu.Unlock()

	impressClient = impr
}
