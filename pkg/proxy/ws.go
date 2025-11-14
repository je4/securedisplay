package proxy

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
)

func (srv *SocketServer) ws(ctx *gin.Context) {
	var secureName string
	if ctx.Request.TLS != nil {
		for _, cert := range ctx.Request.TLS.PeerCertificates {
			for _, name := range cert.DNSNames {
				if strings.HasPrefix(name, "ws:") {
					secureName = name[3:]
					break
				}
			}
			if secureName != "" {
				break
			}
		}
	}

	conn, err := srv.upgrade(ctx, 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	name := ctx.Param("name")
	if err := srv.addWSConn(NewConnection(conn, name, secureName != "" && name == secureName), name); err != nil {
		srv.logger.Error().Err(err).Msgf("Failed to add connection %s", name)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to add connection"})
		return
	}
	defer srv.closeWSConn(name)

	for {
		var event = &event.Event{}
		if err := conn.ReadJSON(event); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				srv.logger.Debug().Err(err).Msg("Connection closed by client")
			} else {
				srv.logger.Error().Err(err).Msg("Failed to read message")
			}
			break
		}
		event.Source = secureName
		srv.logger.Debug().Msgf("Received event: %s", event)
		if event.Target != "" {
			srv.logger.Debug().Msgf("Sending event to target %s: %s", event.Target, event)
			if targetConn, ok := srv.getWSConn(name); ok {
				if err := targetConn.Conn.WriteJSON(event); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
						srv.logger.Debug().Err(err).Msg("Connection closed by client")
					} else {
						srv.logger.Error().Err(err).Msgf("Failed to send event to target %s: %s", event.Target, event)
					}
				}
			} else {
				srv.logger.Error().Msgf("Target connection %s not found", event.Target)
			}
		} else {
			eventStruct, err := event.Decode()
			if err != nil {
				srv.logger.Error().Err(err).Msgf("Failed to decode event %s: %s", event.Type, string(event.Data))
			}
			srv.logger.Warn().Msgf("Received event [%s] with no target", eventStruct)
		}
	}
}
