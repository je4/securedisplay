package main

import (
	"flag"

	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/securedisplay/config"
	"github.com/je4/utils/v2/pkg/stashconfig"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

var name = flag.String("name", "", "name of the display")
var proxy = flag.String("proxy", "", "address of the websocket proxy server")
var debug = flag.Bool("debug", false, "debug mode")
var configPath = flag.String("config", "", "path to config file")
var playerURL = flag.String("player", "", "url of the player server")
var noKiosk = flag.Bool("no-kiosk", false, "disable kiosk")

type DisplayConfig struct {
	ProxyAddr string             `toml:"localaddr"`
	Name      string             `toml:"name"`
	PlayerURL string             `toml:"player"`
	Kiosk     bool               `toml:"kiosk"`
	Debug     bool               `toml:"debug"`
	ClientTLS loader.Config      `toml:"clienttls"`
	Log       stashconfig.Config `toml:"log"`
}

func loadConfig() (*DisplayConfig, error) {
	flag.Parse()
	cfg := &DisplayConfig{}
	// fill the default values
	if _, err := toml.Decode(string(config.DisplayToml), cfg); err != nil {
		return nil, errors.Wrap(err, "failed to load default config from")
	}
	if *configPath != "" {
		// enhance it with the external file
		if _, err := toml.DecodeFile(*configPath, cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to load config from %s", *configPath)
		}
	}
	flag.VisitAll(func(f *flag.Flag) {
		switch f.Name {
		case "debug":
			cfg.Debug = *debug
		case "proxy":
			cfg.ProxyAddr = *proxy
		case "player":
			cfg.PlayerURL = *playerURL
		case "no-kiosk":
			cfg.Kiosk = !*noKiosk
		}
	})

	return cfg, nil
}
