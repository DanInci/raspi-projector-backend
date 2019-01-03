package main

import (
	controller "github.com/DanInci/raspberry-projector/controller"
	impress "github.com/DanInci/raspberry-projector/impress"
	log "github.com/apsdehal/go-logger"
	configure "github.com/paked/configure"
	http "net/http"
	// qrcode "github.com/skip2/go-qrcode"
	os "os"
	signal "os/signal"
	syscall "syscall"
)

var (
	conf                = configure.New()
	logger              = setupLogger()
	libreOfficePath     = conf.String("libre-office-path", "/Applications/LibreOffice.app/Contents/MacOS/soffice", "Path for LibreOffice")
	libreRemoteURL      = conf.String("libre-remote-url", "ws://localhost:1599", "The default URL for libre remote connection")
	libreRemoteName     = conf.String("libre-remote-name", "WebServer", "The name for the remote")
	libreRemotePIN      = conf.String("libre-remote-pin", "13579", "The PIN for the remote connection")
	libreMaxControllers = conf.Int("libre-max-controllers", 10, "The maximum number of slideshow controllers allowed")
	libreMaxTimeout     = conf.Int("libre-max-timeout", 6000, "The number of seconds the slideshow owner is allowed to be disconnected before drop")
	maxUploadSize       = conf.Int("max-upload-size", 1024*1024*10, "The maximum upload size for files")
	filesDirectory      = conf.String("files-directory", "uploads", "The directory where the uploaded files would be saved")
	httpAddr            = conf.String("http-addr", ":8080", "Address for http server")
)

func init() {
	impress.Logger = logger
	controller.Logger = logger
}

func setupConfigs() {
	conf.Use(configure.NewEnvironment())
	conf.Use(configure.NewFlag())
}

func setupLogger() *log.Logger {
	log, err := log.New("projector-service", 1, os.Stdout)
	if err != nil {
		panic(err)
	}
	log.SetFormat("[%{time}] %{message}")
	return log
}

func setupHTTPServer() *http.Server {
	httpServer := &http.Server{Addr: *httpAddr}

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		controller.GetStats(w, r)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		controller.UploadPPT(w, r, *filesDirectory, int64(*maxUploadSize))
	})

	http.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		controller.ServeImpressController(w, r)
	})

	return httpServer
}

func setupImpress() {
	impress.Configure(*libreOfficePath, *libreRemoteURL, *libreRemoteName, *libreRemotePIN, *libreMaxControllers, *libreMaxTimeout)
}

func main() {
	logger.InfoF("Process started with PID %d", os.Getpid())

	setupConfigs()
	conf.Parse()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	setupImpress()
	httpServer := setupHTTPServer()
	logger.InfoF("Starting http server on localhost%s...", httpServer.Addr)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil {
			logger.CriticalF("Error starting http server: %v", err)
			logger.Fatal("Shutting down...")
		}
	}()

	<-c
	logger.Notice("Received shutdown signal")
	controller.Terminate(httpServer)
}
