package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/securedisplay/pkg/genericplayer"
	"github.com/je4/trustutil/v2/pkg/certutil"
	"github.com/je4/utils/v2/pkg/zLogger"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

func main() {

	conf, err := loadConfig()
	if err != nil {
		panic(fmt.Sprintf("Error loading config: %v", err))
	}
	var loggerTLSConfig *tls.Config
	var loggerLoader io.Closer
	if conf.Log.Stash.TLS != nil {
		loggerTLSConfig, loggerLoader, err = loader.CreateClientLoader(conf.Log.Stash.TLS, nil)
		if err != nil {
			log.Fatalf("cannot create stash client loader: %v", err)
		}
		defer loggerLoader.Close()
	}

	_logger, _logstash, _logfile, err := ublogger.CreateUbMultiLoggerTLS(conf.Log.Level, conf.Log.File,
		ublogger.SetDataset(conf.Log.Stash.Dataset),
		ublogger.SetLogStash(conf.Log.Stash.LogstashHost, conf.Log.Stash.LogstashPort, conf.Log.Stash.Namespace, conf.Log.Stash.LogstashTraceLevel),
		ublogger.SetTLS(conf.Log.Stash.TLS != nil),
		ublogger.SetTLSConfig(loggerTLSConfig),
	)
	if err != nil {
		log.Fatalf("cannot create logger: %v", err)
	}
	if _logstash != nil {
		defer _logstash.Close()
	}
	if _logfile != nil {
		defer _logfile.Close()
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("cannot get hostname: %v", err)
	}
	l2 := _logger.With().Timestamp().Str("host", hostname).Logger() //.Output(output)
	var logger zLogger.ZLogger = &l2

	certutil.AddDefaultDNSNames("ws:" + conf.Name)
	clientTLSConfig, clientLoader, err := loader.CreateClientLoader(&conf.ClientTLS, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot create client loader")
	}
	defer clientLoader.Close()
	ca, err := clientLoader.GetCA()
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot get CA")
	}
	clientTLSConfig.RootCAs = ca

	wsPath, err := url.JoinPath(conf.ProxyAddr, conf.Name)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create websocket path")
		return
	}
	logger.Info().Msgf("Connecting to websocket proxy server at %s", wsPath)

	wsDialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		TLSClientConfig:  clientTLSConfig,
	}
	c, _, err := wsDialer.Dial(wsPath, nil)
	if err != nil {
		logger.Error().Err(err).Msgf("Failed to connect to websocket proxy server with %s", wsPath)
		return
	}
	defer c.Close()

	comm := client.NewCommunication(c, *name, logger)
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
	jsonBytes, err := json.Marshal("core")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal json")
	}
	if err := comm.Send(&event.Event{
		Type:   event.TypeAttach,
		Source: *name,
		Target: "",
		Token:  "",
		Data:   jsonBytes,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to send event")
	}
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
		"enable-features":                     "AutoplayIgnoreWebAudio",
		"disable-features":                    "InfiniteSessionRestore,TranslateUI,PreloadMediaEngagementData,MediaEngagementBypassAutoplayPolicies",
		"autoplay-policy":                     "no-user-gesture-required",
		//"no-first-run":                        true,
		"enable-fullscreen-toolbar-reveal": false,
		"useAutomationExtension":           false,
		"enable-automation":                false,
		"mute-audio":                       false,
	}
	if *noKiosk {
		opts["kiosk"] = false
	}
	br, err := browser.NewBrowser(opts, logger, func(s string, i ...interface{}) {
		logger.Debug().Msgf("browser: %s - %v", s, i)
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create browser")
	}

	playerFullPath, err := url.JoinPath(conf.PlayerURL, conf.Name)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create player path")
	}
	playerU, err := url.Parse(playerFullPath)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to parse player URL")
	}

	player := genericplayer.NewPlayer(context.Background(), playerU, br, comm, logger)
	_ = player

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM, syscall.SIGTERM)
	<-sigint
	logger.Info().Msg("Received shutdown signal")
}
