package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/je4/securedisplay/pkg/server"
	"github.com/rs/zerolog"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

func main() {
	flag.Parse()
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	logger.Info().Msgf("Starting server on %s", *addr)

	srv, err := server.NewSocketServer(*addr, &logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create server")
		return
	}
	if err := srv.Start(nil); err != nil {
		logger.Error().Err(err).Msg("Failed to start server")
		return
	}
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	signal.Notify(sigint, syscall.SIGTERM)
	signal.Notify(sigint, syscall.SIGKILL)
	<-sigint
	logger.Info().Msg("Received shutdown signal")
	if err := srv.Stop(); err != nil {
		logger.Error().Err(err).Msg("Failed to stop server")
	}
}
