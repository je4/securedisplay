package proxy

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
)

func (srv *SocketServer) ws(ctx *gin.Context) {
	var secureName string
	var name = ctx.Param("name")
	if ctx.Request.TLS != nil {
		for _, cert := range ctx.Request.TLS.PeerCertificates {
			for _, dnsName := range cert.DNSNames {
				if strings.HasPrefix(name, "ws:") {
					secureName = dnsName[3:]
					break
				}
			}
			if secureName != "" {
				break
			}
		}
	} else {
		if !srv.debug {
			srv.logger.Error().Msgf("No TLS certificate found for client %s[%s]", name, ctx.Request.RemoteAddr)
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("No TLS certificate found for client %s[%s]", name, ctx.Request.RemoteAddr)})
			return
		}
	}

	if secureName != "" && secureName != name {
		srv.logger.Error().Msgf("'%s' does not match tls name '%s'", name, secureName)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": http.StatusText(http.StatusBadRequest), "message": fmt.Sprintf("'%s' does not match tls name '%s'", name, secureName)})
		return
	}
	conn, err := srv.upgrade(ctx, name, 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	wsConn := NewConnection(conn, name, secureName != "" && name == secureName)
	if err := srv.addWSConn(wsConn); err != nil {
		srv.logger.Error().Err(err).Msgf("Failed to add connection %s", name)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to add connection"})
		return
	}
	defer srv.closeWSConn(wsConn)

	for {
		var evt = &event.Event{}
		if err := conn.ReadJSON(evt); err != nil {
			if websocket.IsCloseError(errors.Cause(err), websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				srv.logger.Debug().Err(err).Msg("Connection closed by client")
			} else {
				srv.logger.Error().Err(err).Msg("Failed to read message")
			}
			break
		}
		//event.Source = secureName
		srv.logger.Debug().Msgf("Received event: %s", evt)
		if evt.Target != "" {
			srv.logger.Debug().Msgf("Sending event to target %s: %s", evt.Target, evt)
			if targetConn, ok := srv.getWSConn(name); ok {
				if err := targetConn.Conn.WriteJSON(evt); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
						srv.logger.Debug().Err(err).Msg("Connection closed by client")
					} else {
						srv.logger.Error().Err(err).Msgf("Failed to send event to target %s: %s", evt.Target, evt)
					}
				}
			} else {
				srv.logger.Error().Msgf("Target connection %s not found", evt.Target)
			}
		} else {
			switch evt.Type {
			case event.TypeAttach:
				if name != evt.GetSource() {
					srv.logger.Error().Msgf("Attach event for %s on %s not allowed", evt.GetSource(), name)
					continue
				}
				data, err := evt.GetData()
				if err != nil {
					srv.logger.Error().Err(err).Msg("Failed to get data for attach event")
					continue
				}
				group := data.(string)
				srv.AddToGroup(name, group)
			case event.TypeDetach:

			}
			eventStruct, err := event.Decode()
			if err != nil {
				srv.logger.Error().Err(err).Msgf("Failed to decode event %s: %s", event.Type, string(event.Data))
			}
			srv.logger.Warn().Msgf("Received event [%s] with no target", eventStruct)
		}
	}
}
