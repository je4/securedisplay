package main

import (
	"github.com/je4/utils/v2/pkg/stashconfig"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

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
