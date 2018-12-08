package main

import (
	logger "github.com/apsdehal/go-logger"
	framebuffer "github.com/gonutz/framebuffer"
	qrcode "github.com/skip2/go-qrcode"
	"image"
	"image/draw"
	"os"
)

func setupLogger() *logger.Logger {
	log, err := logger.New("projector-service", 1, os.Stdout)
	if err != nil {
		panic(err)
	}
	log.SetFormat("[%{time}] {%message}")
	return log
}

func main() {
	var qrCode *qrcode.QRCode
	var logger *logger.Logger

	logger = setupLogger()

	qrCode, err := qrcode.New("WIFI:T:WPA;S:Dani's Raspberry;P:123456987asd;;", qrcode.Highest)
	if err != nil {
		logger.Error("Failed to generate QR code")
		panic(err)
	}

	fb, err := framebuffer.Open("/dev/fb")
	if err != nil {
		logger.Error("Failed to open framebuffer")
		panic(err)
	}
	defer fb.Close()

	qrImg := qrCode.Image(256)
	draw.Draw(fb, fb.Bounds(), qrImg, image.ZP, draw.Src)
}
