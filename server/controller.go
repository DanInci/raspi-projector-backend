package server

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

const DEFAULT_MAX_UPLOAD_SIZE = 1024
const DEFAULT_UPLOAD_DIRECTORY = "upload"
const OWNER_UUID = "ownerUUID"

var Logger *log.Logger
var MaxUploadSize int = DEFAULT_MAX_UPLOAD_SIZE
var UploadDirectory string = DEFAULT_UPLOAD_DIRECTORY

var impressClient *impress.ImpressClient
var mu sync.Mutex = sync.Mutex{}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !isSlideShowRunning() {
		writeError(w, "Slideshow is not running", http.StatusNotFound)
		return
	}

	stats := getImpressClient().GetStats()
	response, err := encodeImpressStats(&stats)
	if err != nil {
		Logger.ErrorF("Error encoding stats: %v", err)
		writeError(w, "Failed to get encode stats", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func UploadPPT(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(MaxUploadSize))
	if err := r.ParseMultipartForm(int64(MaxUploadSize)); err != nil {
		Logger.InfoF("err: %v", err)
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

	uploadFolderPath := filepath.Join(filepath.Dir(os.Args[0]), UploadDirectory)
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

	ownerUUID, err := r.Cookie(OWNER_UUID)
	isOwner := false
	if err != nil {
		isOwner = isSlideShowOwnerUUID(ownerUUID.Value)
	}

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
