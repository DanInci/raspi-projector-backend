package impress

import (
	bufio "bufio"
	errors "errors"
	log "github.com/apsdehal/go-logger"
	net "net"
	url "net/url"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	runtime "runtime"
	strconv "strconv"
	strings "strings"
	sync "sync"
	syscall "syscall"
	time "time"
)

var Logger *log.Logger

var currentConfig *configuration = &DefaultConfig

const (
	PAIRED     = "LO_SERVER_SERVER_PAIRED"
	VALIDATING = "LO_SERVER_VALIDATING_PIN"

	SLIDE_SHOW_INFO     = "slideshow_info"
	SLIDE_SHOW_STARTED  = "slideshow_started"
	SLIDE_SHOW_FINISHED = "slideshow_finished"
	SLIDE_UPDATED       = "slide_updated"
	SLIDE_PREVIEW       = "slide_preview"
	SLIDE_NOTES         = "slide_notes"
)

var DefaultConfig = configuration{
	libreOfficePath: "soffice",
	remoteName:      "Remote",
	remotePIN:       "12345",
	maxControllers:  10,
	ownerTimeout:    60,
}

type ImpressStats struct {
	Name           string
	Status         []string
	Controllers    int
	MaxControllers int
	IsOwnerPresent bool
	OwnerTimeout   int
}

type ImpressClient struct {
	conn         net.Conn
	configs      configuration
	presentation *presentation
	stats        ImpressStats
	controllers  []*ImpressController
	isTerminated bool
	shutdown     chan bool
	requests     chan []string
	messages     chan []string
	register     chan *ImpressController
	unregister   chan *ImpressController
	ticker       *time.Ticker
	mu           sync.Mutex
}

type configuration struct {
	libreOfficePath string
	remoteURL       string
	remoteName      string
	remotePIN       string
	maxControllers  int
	ownerTimeout    int
}

type presentation struct {
	uuid     string
	filePath string
	command  *exec.Cmd
}

func Configure(librePath string, remoteURL string, remoteName string, remotePIN string, maxControllers int, ownerTimeout int) {
	currentConfig = &configuration{
		libreOfficePath: librePath,
		remoteURL:       remoteURL,
		remoteName:      remoteName,
		remotePIN:       remotePIN,
		maxControllers:  maxControllers,
		ownerTimeout:    ownerTimeout,
	}
}

func NewClient() *ImpressClient {
	client := &ImpressClient{
		conn:         nil,
		configs:      *currentConfig,
		presentation: nil,
		stats:        ImpressStats{Name: "", Status: make([]string, 0), Controllers: 0, MaxControllers: currentConfig.maxControllers, IsOwnerPresent: false, OwnerTimeout: currentConfig.ownerTimeout},
		controllers:  make([]*ImpressController, 0),
		isTerminated: false,
		shutdown:     make(chan bool),
		requests:     make(chan []string),
		messages:     make(chan []string),
		register:     make(chan *ImpressController),
		unregister:   make(chan *ImpressController),
		ticker:       nil,
		mu:           sync.Mutex{},
	}
	go client.handleRegistrations()
	return client
}

func (impr *ImpressClient) StartPresentation(uuid string, path string) error {
	cmd := exec.Command(impr.configs.libreOfficePath, "--invisible", "--norestore", "--show", path)

	if err := cmd.Start(); err != nil {
		return err
	} else {
		impr.presentation = &presentation{
			uuid:     uuid,
			filePath: path,
			command:  cmd,
		}
		return nil
	}
}

func (impr *ImpressClient) OpenConnection() error {
	u, err := url.Parse(impr.configs.remoteURL)
	if err != nil {
		return err
	}

	rawConn, err := net.Dial("tcp", u.Host)
	i := 1
	ticker := time.NewTicker(3 * time.Second)
	for err != nil && i < 5 {
		Logger.ErrorF("Attempt no %d to connect to impress failed. Retrying...", i)
		select {
		case <-ticker.C:
			rawConn, err = net.Dial("tcp", u.Host)
		}
		i++
	}
	ticker.Stop()
	if err != nil {
		return err
	}

	if runtime.GOOS == "darwin" {
		time.Sleep(5 * time.Second)
	}
	err1 := sendRequest([]string{PAIR_WITH_SERVER, impr.configs.remoteName, impr.configs.remotePIN}, rawConn)
	if err1 != nil {
		rawConn.Close()
		return err1
	}

	messages, err2 := readMessage(rawConn)
	if err2 != nil {
		rawConn.Close()
		return err2
	}
	if messages[0] == VALIDATING {
		rawConn.Close()
		return errors.New("Remote server not authorised")
	} else if messages[0] != PAIRED {
		rawConn.Close()
		return errors.New("Failed connection handshake")
	}

	impr.conn = rawConn
	return nil
}

func (impr *ImpressClient) CloseConnection() {
	if impr.conn != nil {
		if err := impr.conn.Close(); err != nil {
			Logger.ErrorF("Error closing remote connection: %v", err)
		}
		impr.conn = nil
	}
}

func (impr *ImpressClient) StopPresentation() {
	if impr.presentation != nil {
		if runtime.GOOS == "windows" {
			pid := strconv.Itoa(impr.presentation.command.Process.Pid)
			if err := exec.Command("taskkill", "/F", "/T", "/PID", pid).Run(); err != nil {
				Logger.ErrorF("Error stopping presenation: %v", err)
			}
		} else {
			if err := impr.presentation.command.Process.Signal(syscall.SIGTERM); err != nil {
				Logger.ErrorF("Error stopping presenation: %v", err)
			}
		}
		if err := os.RemoveAll(filepath.Dir(impr.presentation.filePath)); err != nil {
			Logger.ErrorF("Failed to remove file: %v", err)
		}
		impr.presentation = nil
	}
}

func (impr *ImpressClient) GetPresentationUUID() string {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	if impr.presentation != nil {
		return impr.presentation.uuid
	} else {
		return ""
	}
}

func (impr *ImpressClient) GetPresentationPath() string {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	if impr.presentation != nil {
		return impr.presentation.filePath
	} else {
		return ""
	}
}

func (impr *ImpressClient) GetStats() ImpressStats {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	return impr.stats
}

func (impr *ImpressClient) IsTerminated() bool {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	return impr.isTerminated
}

func (impr *ImpressClient) HasControllerSpace() bool {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	return impr.stats.Controllers < impr.configs.maxControllers
}

func (impr *ImpressClient) ListenAndServe() {
	go impr.listenForMessages()
	go impr.serveRequests()
	Logger.Info("Impress client started listening & serving")

	stats := impr.GetStats()
	if !stats.IsOwnerPresent {
		impr.mu.Lock()

		Logger.InfoF("Waiting %d seconds for owner to join...", impr.configs.ownerTimeout)
		impr.ticker = impr.waitForOwner(time.Duration(impr.configs.ownerTimeout) * time.Second)

		impr.mu.Unlock()
	}
}

func (impr *ImpressClient) Terminate() {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	if !impr.isTerminated {
		impr.isTerminated = true

		Logger.Notice("Impress client received terminate signal. Shutting down")
		if impr.ticker != nil {
			impr.ticker.Stop()
		}
		for _, controller := range impr.controllers {
			controller.send <- []string{SLIDE_SHOW_FINISHED}
			close(controller.send)
		}
		close(impr.shutdown)
		impr.CloseConnection()
		impr.StopPresentation()
	}
}

func (impr *ImpressClient) handleRegistrations() {
	for {
		select {
		case controller := <-impr.register:
			impr.mu.Lock()

			if impr.stats.Controllers < impr.configs.maxControllers {
				impr.controllers = append(impr.controllers, controller)
				if controller.IsOwner() {
					Logger.Info("Owner joined the presentation")
					impr.ticker.Stop()
					impr.stats.IsOwnerPresent = true
				}
				impr.stats.Controllers++
			} else {
				Logger.Info("The maximum number of controllers was reached")
			}
			controller.send <- impr.stats.Status

			impr.mu.Unlock()
		case controller := <-impr.unregister:
			for i, contr := range impr.controllers {
				if contr == controller {
					impr.mu.Lock()

					impr.controllers = append(impr.controllers[:i], impr.controllers[i+1:]...)
					if contr.IsOwner() {
						Logger.InfoF("Presentation owner has left. Waiting %d seconds for him to come back...", impr.configs.ownerTimeout)
						impr.ticker = impr.waitForOwner(time.Duration(impr.configs.ownerTimeout) * time.Second)
						impr.stats.IsOwnerPresent = false
					}
					impr.stats.Controllers--

					impr.mu.Unlock()
					close(contr.send)
					break
				}
			}
		case <-impr.shutdown:
			return
		}
	}
}

func (impr *ImpressClient) waitForOwner(duration time.Duration) *time.Ticker {
	ticker := time.NewTicker(duration)
	go func(shutdown chan bool) {
		select {
		case <-ticker.C:
			impr.Terminate()
		case <-shutdown:
			return
		}
	}(impr.shutdown)
	return ticker
}

func (impr *ImpressClient) listenForMessages() {
	for {
		message, err := readMessage(impr.conn)
		if err != nil {
			if !impr.isTerminated {
				Logger.ErrorF("Error reading Impress message: %v", err)
				Logger.Critical("Impress client stopped listening for messages")
			}
			break
		}
		if ok := checkValidMessage(message); ok {
			impr.messages <- message
		}
	}
}

func checkValidMessage(message []string) bool {
	switch message[0] {
	case PAIRED, VALIDATING, SLIDE_SHOW_INFO, SLIDE_SHOW_FINISHED, SLIDE_SHOW_STARTED, SLIDE_UPDATED:
		return true
	default:
		return false
	}
}

func (impr *ImpressClient) serveRequests() {
	for {
		select {
		case message := <-impr.messages:
			for _, controller := range impr.controllers {
				select {
				case controller.send <- message:
				}
			}
			switch message[0] {
			case SLIDE_SHOW_INFO:
				impr.stats.Name = message[1]
			case SLIDE_SHOW_FINISHED, SLIDE_SHOW_STARTED, SLIDE_UPDATED:
				impr.updateStatus(message)
			}
		case request := <-impr.requests:
			err := sendRequest(request, impr.conn)
			if request[0] == PRESENTATION_STOP {
				impr.Terminate()
				break
			}
			if err != nil {
				Logger.ErrorF("Error writing Impress request: %v", err)
				Logger.Critical("Impress client stopped serving controller requests")
				break
			}
		case <-impr.shutdown:
			return
		}
	}
}

func (impr *ImpressClient) updateStatus(messages []string) {
	impr.mu.Lock()
	defer impr.mu.Unlock()

	switch messages[0] {
	case SLIDE_SHOW_FINISHED:
		impr.stats.Status = []string{SLIDE_SHOW_FINISHED}
	case SLIDE_SHOW_STARTED:
		impr.stats.Status = []string{SLIDE_SHOW_STARTED, messages[1], messages[2]}
	case SLIDE_UPDATED:
		if impr.stats.Status != nil && impr.stats.Status[0] == SLIDE_SHOW_STARTED {
			impr.stats.Status[2] = messages[1]
		}
	}
}

func readMessage(conn net.Conn) ([]string, error) {
	reader := bufio.NewReader(conn)
	messages := make([]string, 0)
	for {
		bytes, _, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}

		message := string(bytes)
		if message == "" {
			break
		}

		messages = append(messages, message)
	}
	return messages, nil
}

func sendRequest(messages []string, conn net.Conn) error {
	writer := bufio.NewWriter(conn)
	formattedMessage := strings.Join(messages, "\n") + "\n\n"
	_, err := writer.WriteString(formattedMessage)
	if err != nil {
		return err
	}
	writer.Flush()
	return nil
}
