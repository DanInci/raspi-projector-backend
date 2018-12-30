package controller

import (
	json "encoding/json"
	impress "github.com/DanInci/raspberry-projector/impress"
	log "github.com/apsdehal/go-logger"
	websocket "github.com/gorilla/websocket"
	http "net/http"
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

func UploadPPT(w http.ResponseWriter, r *http.Request, remoteName string, remotePIN string, maxControllers int, ownerTimeout int) {
	w.Header().Set("Content-Type", "application/json")
	if !isSlideShowRunning() {
		uuid := generateUUID()

		client := impress.NewClient(uuid, maxControllers, ownerTimeout)
		err1 := client.OpenConnection(remoteName, remotePIN)
		if err1 != nil {
			Logger.CriticalF("Failed to connect to impress: %s", err1)
			w.WriteHeader(http.StatusInternalServerError)
			writeError(w, "Slideshow failed to start")
		}
		client.ListenAndServe()
		setImpressClient(client)

		toEncode := make(map[string]interface{})
		toEncode["ownerUUID"] = uuid
		encoded, _ := json.Marshal(toEncode)
		w.WriteHeader(http.StatusCreated)
		w.Write(encoded)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "Slideshow already running")
	}

}

func ServeImpressController(w http.ResponseWriter, r *http.Request) {
	if !isSlideShowRunning() {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "Slideshow not running")
		return
	}

	client := getImpressClient()
	hasControllerSpace := client.HasControllerSpace()
	if !hasControllerSpace {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "Slideshow has reached the maximum number of controllers")
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

func writeError(w http.ResponseWriter, message string) {
	toEncode := make(map[string]interface{})
	toEncode["error"] = message
	encoded, _ := json.Marshal(toEncode)
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
