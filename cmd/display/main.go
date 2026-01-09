package main

import (
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
)

var name = flag.String("name", "display01", "name of the display")
var proxy = flag.String("proxy", "ws://localhost:7081/ws", "address of the websocket proxy server")
var playerURL = flag.String("player", "http://localhost:7081/roundaudio", "url of the player server")
var noKiosk = flag.Bool("no-kiosk", false, "disable kiosk")

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
	if err := comm.Send(&event.Event{
		Type:   event.TypeStringMessage,
		Source: "",
		Target: "core",
		Token:  "",
		Data:   jsonBytes,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to send event")
	}
	if err := comm.NTP(); err != nil {
		logger.Error().Err(err).Msg("Failed to send NTP")
	}
	/*
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
		if *noKiosk {
			opts["kiosk"] = false
		}
		br, err := browser.NewBrowser(opts, &logger, func(s string, i ...interface{}) {
			logger.Debug().Msgf("browser: %s - %v", s, i)
		})
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to create browser")
		}

		playerFullPath, err := url.JoinPath(*playerURL, *name)
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to create player path")
		}
		playerU, err := url.Parse(playerFullPath)
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to parse player URL")
		}

			player := genericplayer.NewPlayer(context.Background(), playerU, br, comm, &logger)
			_ = player

	*/
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGTERM)
	<-sigint
	logger.Info().Msg("Received shutdown signal")
}
