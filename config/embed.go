package config

import _ "embed"

//go:embed proxydefault.toml
var ProxyToml []byte

//go:embed displaydefault.toml
var DisplayToml []byte
