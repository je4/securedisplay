package main

import (
	"flag"
	"runtime"

	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/securedisplay/config"
	"github.com/je4/utils/v2/pkg/stashconfig"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

var addr = flag.String("addr", "localhost:8080", "internal http service address")
var ext = flag.String("ext", "localhost:8080", "external http service address")
var ntpServer = flag.String("ntp", "localhost", "ntp server address")
var numWorker = flag.Int("workers", runtime.NumCPU(), "number of workers")
var debug = flag.Bool("debug", false, "debug mode")
var webFolder = flag.String("web", "", "web folder to serve the display from")
var configPath = flag.String("config", "", "path to config file")

type ProxyConfig struct {
	LocalAddr             string             `toml:"localaddr"`
	ExternalAddr          string             `toml:"externaladdr"`
	NTP                   string             `toml:"ntp"`
	NumWorkers            int                `toml:"num_workers"`
	Debug                 bool               `toml:"debug"`
	WebFolder             string             `toml:"web_folder"`
	MiniresolverClientTLS loader.Config      `toml:"miniresolverclienttls"`
	ServerTLS             loader.Config      `toml:"servertls"`
	Log                   stashconfig.Config `toml:"log"`
}

func loadConfig() (*ProxyConfig, error) {
	flag.Parse()
	cfg := &ProxyConfig{}
	// fill the default values
	if _, err := toml.Decode(string(config.ProxyToml), cfg); err != nil {
		return nil, errors.Wrap(err, "failed to load default config from")
	}
	if *configPath != "" {
		// enhance with the external file
		if _, err := toml.DecodeFile(*configPath, cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to load config from %s", *configPath)
		}
	}
	flag.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		case "web":
			cfg.WebFolder = *webFolder
		case "debug":
			cfg.Debug = true
		case "workers":
			cfg.NumWorkers = *numWorker
		case "ntp":
			cfg.NTP = *ntpServer
		case "addr":
			cfg.LocalAddr = *addr
		case "ext":
			cfg.ExternalAddr = *ext
		}
	})

	return cfg, nil
}
