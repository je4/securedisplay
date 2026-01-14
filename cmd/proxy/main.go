package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/je4/securedisplay/pkg/proxy"
	"github.com/je4/utils/v2/pkg/zLogger"
	ublogger "gitlab.switch.ch/ub-unibas/go-ublogger/v2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

func main() {

	conf, err := loadConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %s", err.Error()))
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

	serverTLSConfig, serverLoader, err := loader.CreateServerLoader(true, &conf.ServerTLS, nil, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot create server loader")
	}
	defer serverLoader.Close()
	serverTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert

	var webFS fs.FS
	webFS = os.DirFS(conf.WebFolder)

	staticFS, err := fs.Sub(webFS, "static")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create static file system")
		return
	}
	templateFS, err := fs.Sub(webFS, "templates")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create template file system")
	}

	srv, err := proxy.NewSocketServer(conf.LocalAddr, conf.ExternalAddr, conf.NumWorkers, conf.NTP, staticFS, templateFS, conf.Debug, logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create server")
		return
	}
	if err := srv.Start(serverTLSConfig); err != nil {
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
