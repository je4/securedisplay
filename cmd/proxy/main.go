package main

import (
	"flag"
	"io/fs"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/je4/securedisplay/data"
	"github.com/je4/securedisplay/pkg/proxy"
	"github.com/rs/zerolog"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var ntpServer = flag.String("ntp", "localhost", "ntp server address")
var numWorker = flag.Int("workers", runtime.NumCPU(), "number of workers")
var debug = flag.Bool("debug", false, "debug mode")
var webFolder = flag.String("web", "", "web folder to serve the display from")

func main() {
	flag.Parse()
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	logger.Info().Msgf("Starting server on %s", *addr)

	var webFS fs.FS
	if *webFolder != "" {
		webFS = os.DirFS(*webFolder)
	} else {
		webFS = data.FS
	}
	staticFS, err := fs.Sub(webFS, "static")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create static file system")
		return
	}
	templateFS, err := fs.Sub(webFS, "templates")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create template file system")
	}

	srv, err := proxy.NewSocketServer(*addr, *numWorker, *ntpServer, staticFS, templateFS, *debug, &logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create server")
		return
	}
	if err := srv.Start(nil); err != nil {
		logger.Error().Err(err).Msg("Failed to start server")
		return
	}
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM)
	<-sigint
	logger.Info().Msg("Received shutdown signal")
	if err := srv.Stop(); err != nil {
		logger.Error().Err(err).Msg("Failed to stop server")
	}
}
