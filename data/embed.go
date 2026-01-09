package data

import "embed"

//go:embed static/js/reconnecting-websocket.js static/js/reconnecting-websocket.min.js
//go:embed templates/roundaudio.gohtml templates/echo.gohtml templates/control.gohtml
var FS embed.FS
