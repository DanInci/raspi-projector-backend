package main

import (
	fmt "fmt"
	impress "github.com/DanInci/raspberry-projector/impress"
	server "github.com/DanInci/raspberry-projector/server"
	log "github.com/apsdehal/go-logger"
	mux "github.com/gorilla/mux"
	configure "github.com/paked/configure"
	qrcode "github.com/skip2/go-qrcode"
	http "net/http"
	os "os"
	signal "os/signal"
	filepath "path/filepath"
	time "time"
)

var (
	conf                = configure.New()
	logger              = setupLogger()
	libreOfficePath     = conf.String("libre-office-path", "soffice", "Path for LibreOffice")
	libreRemoteURL      = conf.String("libre-remote-url", "ws://localhost:1599", "The default URL for libre remote connection")
	libreRemoteName     = conf.String("libre-remote-name", "WebServer", "The name for the remote")
	libreRemotePIN      = conf.String("libre-remote-pin", "13579", "The PIN for the remote connection")
	libreMaxControllers = conf.Int("libre-max-controllers", 10, "The maximum number of slideshow controllers allowed")
	libreMaxTimeout     = conf.Int("libre-max-timeout", 60, "The number of seconds the slideshow owner is allowed to be disconnected before drop")
	maxUploadSize       = conf.Int("max-upload-size", 1024*1024*10, "The maximum upload size for files")
	uploadsDirectory    = conf.String("uploads-directory", "uploads", "The directory where the uploaded files would be saved")
	qrDirectory         = conf.String("qr-directory", "www-qr", "The directory from where the qr files are served")
	clientDirectory     = conf.String("client-directory", "www-client", "The directory from where the qr files are served")
	networkSSID         = conf.String("network-ssid", "Dani's Raspberry", "The network SSID used to generate the connection QR Code")
	networkPass         = conf.String("network-pass", "123456987asd", "The network password used to generate the connection QR Code")
	httpAddr            = conf.String("http-addr", "0.0.0.0:8080", "Address for http server")
)

func init() {
	impress.Logger = logger
	server.Logger = logger
}

func setupConfigs() {
	conf.Use(configure.NewHCLFromFile("application.conf"))
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

func setupImpress() {
	impress.Configure(*libreOfficePath, *libreRemoteURL, *libreRemoteName, *libreRemotePIN, *libreMaxControllers, *libreMaxTimeout)
}

func setupHTTPServer() *http.Server {
	r := mux.NewRouter()

	// r.Use(server.CorsMiddleware)
	// r.Use(server.LoggingMiddleware)

	r.HandleFunc("/stats", server.GetStats).Methods("GET")

	r.HandleFunc("/upload", server.UploadPPT).Methods("POST")

	r.HandleFunc("/control", server.ServeImpressController).Methods("GET")

	r.PathPrefix("/client").Handler(http.StripPrefix("/client", server.NewStaticServer(fmt.Sprintf("./%s", *clientDirectory))))

	r.PathPrefix("/qr").Handler(http.StripPrefix("/qr", server.NewStaticServer(fmt.Sprintf("./%s", *qrDirectory))))

	httpServer := &http.Server{
		Addr:         *httpAddr,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}
	server.MaxUploadSize = *maxUploadSize
	server.UploadDirectory = *uploadsDirectory

	return httpServer
}

func generateQRCode() error {
	content := fmt.Sprintf("WIFI:S:%s;T:WPA;P:%s;;", *networkSSID, *networkPass)
	qrcode, err := qrcode.Encode(content, qrcode.Highest, 512)
	if err != nil {
		return err
	}

	uploadFolderPath := filepath.Join(filepath.Dir(os.Args[0]), *qrDirectory, "assets")
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

func main() {
	logger.InfoF("Process started with PID %d", os.Getpid())

	setupConfigs()
	conf.Parse()

	if err := generateQRCode(); err != nil {
		logger.CriticalF("Failed to generate connection QR Code: %v", err)
		logger.Fatal("Shutting down...")
	}

	setupImpress()

	httpServer := setupHTTPServer()
	logger.InfoF("Starting http server on %s...", httpServer.Addr)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil {
			logger.CriticalF("Error starting http server: %v", err)
			logger.Fatal("Shutting down...")
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	logger.Notice("Received shutdown signal")
	server.Terminate(httpServer)
	os.Exit(0)
}
