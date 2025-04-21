package server

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
	"time"
)

func (srv *SocketServer) ws(ctx *gin.Context) {
	conn, err := srv.upgrade(ctx, 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	name := ctx.Param("name")
	srv.addWSConn(NewConnection(conn, name, false, map[string][]string{"meta": {"global/guest"}, "content": {"global/guest"}, "preview": {"global/guest"}}), name)
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
			_ = eventStruct
		}
	}
}
