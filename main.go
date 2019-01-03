package main

import (
	fmt "fmt"
	controller "github.com/DanInci/raspberry-projector/controller"
	impress "github.com/DanInci/raspberry-projector/impress"
	log "github.com/apsdehal/go-logger"
	configure "github.com/paked/configure"
	qrcode "github.com/skip2/go-qrcode"
	http "net/http"
	os "os"
	signal "os/signal"
	filepath "path/filepath"
	strings "strings"
	syscall "syscall"
)

var (
	conf                = configure.New()
	logger              = setupLogger()
	libreOfficePath     = conf.String("libre-office-path", "soffice", "Path for LibreOffice")
	libreRemoteURL      = conf.String("libre-remote-url", "ws://localhost:1599", "The default URL for libre remote connection")
	libreRemoteName     = conf.String("libre-remote-name", "WebServer", "The name for the remote")
	libreRemotePIN      = conf.String("libre-remote-pin", "13579", "The PIN for the remote connection")
	libreMaxControllers = conf.Int("libre-max-controllers", 10, "The maximum number of slideshow controllers allowed")
	libreMaxTimeout     = conf.Int("libre-max-timeout", 6000, "The number of seconds the slideshow owner is allowed to be disconnected before drop")
	maxUploadSize       = conf.Int("max-upload-size", 1024*1024*10, "The maximum upload size for files")
	filesDirectory      = conf.String("files-directory", "uploads", "The directory where the uploaded files would be saved")
	qrDirectory         = conf.String("qr-directory", "www-qr", "The directory from where the qr files are served")
	networkSSID         = conf.String("network-ssid", "Dani's Raspberry", "The network SSID used to generate the connection QR Code")
	networkPass         = conf.String("network-pass", "123456987asd", "The network password used to generate the connection QR Code")
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

func setupQRCode() error {
	content := fmt.Sprintf("WIFI:S:%s;T:WPA;P:%s;;", *networkSSID, *networkPass)
	qrcode, err := qrcode.Encode(content, qrcode.Highest, 1024)
	if err != nil {
		return err
	}

	uploadFolderPath := filepath.Join(filepath.Dir(os.Args[0]), *qrDirectory, "images")
	os.MkdirAll(uploadFolderPath, os.ModePerm)
	filePath := filepath.Join(uploadFolderPath, "qr.png")

	newFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer newFile.Close()
	if _, err := newFile.Write(qrcode); err != nil {
		return err
	}

	logger.Info("Generated QR Code")
	return nil
}

func setupHTTPServer() *http.Server {
	httpServer := &http.Server{Addr: *httpAddr}

	fileServer := http.FileServer(controller.NewFilesystem(*qrDirectory))
	http.Handle("/qr/", http.StripPrefix(strings.TrimRight("/qr/", "/"), fileServer))

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

	if err := setupQRCode(); err != nil {
		logger.CriticalF("Failed to generate connection QR Code: %v", err)
		logger.Fatal("Shutting down...")
	}

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
