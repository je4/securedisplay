package main

import (
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/audioplayer"
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
)

var name = flag.String("name", "display01", "name of the display")
var proxy = flag.String("proxy", "ws://localhost:7081/ws", "address of the websocket proxy server")

func main() {
	flag.Parse()
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
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

	comm := client.NewCommunication(c, *name, zlogger)
	if err := comm.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to start communication")
		return
	}
	defer func() {
		logger.Info().Msg("Closing communication")
		if err := comm.Stop(); err != nil {
			logger.Error().Err(err).Msg("Failed to stop server")
		}
	}()
	jsonBytes, err := json.Marshal("test")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal json")
	}
	comm.Send(&event.Event{
		Type:   event.TypeStringMessage,
		Source: "",
		Target: "core",
		Token:  "",
		Data:   jsonBytes,
	})
	if err := comm.NTP(); err != nil {
		logger.Error().Err(err).Msg("Failed to send NTP")
	}
	opts := map[string]interface{}{
		"headless":                            false,
		"start-fullscreen":                    true,
		"disable-notifications":               true,
		"disable-infobars":                    true,
		"disable-gpu":                         false,
		"allow-insecure-localhost":            true,
		"enable-immersive-fullscreen-toolbar": true,
		"views-browser-windows":               false,
		"kiosk":                               true,
		"disable-session-crashed-bubble":      true,
		"incognito":                           true,
		//"enable-features":                     "PreloadMediaEngagementData,AutoplayIgnoreWebAudio,MediaEngagementBypassAutoplayPolicies",
		"disable-features": "InfiniteSessionRestore,TranslateUI,PreloadMediaEngagementData,AutoplayIgnoreWebAudio,MediaEngagementBypassAutoplayPolicies",
		//"no-first-run":                        true,
		"enable-fullscreen-toolbar-reveal": false,
		"useAutomationExtension":           false,
		"enable-automation":                false,
	}
	br, err := browser.NewBrowser(opts, &logger, func(s string, i ...interface{}) {
		logger.Debug().Msgf("browser: %s - %v", s, i)
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create browser")
	}

	player := audioplayer.NewPlayer("test", br, comm, &logger)
	_ = player
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGTERM)
	<-sigint
	logger.Info().Msg("Received shutdown signal")
}
