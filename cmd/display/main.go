package main

import (
	"flag"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

var name = flag.String("name", "display01", "name of the display")
var proxy = flag.String("proxy", "http://localhost:8080/ws", "address of the websocket proxy server")

func main() {
	flag.Parse()
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msgf("Starting display with name %s", *name)
	zlogger := zLogger.ZLogger(&logger)

	wsPath, err := url.JoinPath(*proxy, *name)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create websocket path")
		return
	}
	logger.Info().Msgf("Connecting to websocket proxy server at %s", wsPath)

	c, _, err := websocket.DefaultDialer.Dial(wsPath, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to connect to websocket proxy server")
		return
	}
	defer c.Close()

	comm := event.NewCommunication(c, *name, zlogger)
	if err := comm.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to start communication")
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
