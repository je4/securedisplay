package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"emperror.dev/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
)

func (srv *SocketServer) ws(ctx *gin.Context) {
	namesAny, ok := ctx.Get("names")
	if !ok {
		srv.logger.Error().Msg("no names")
		return
	}
	names := namesAny.([]string)

	var name = ctx.Param("name")
	if !slices.Contains(names, "ws:"+name) && !srv.debug {
		srv.logger.Error().Msgf("name %s not in names %v", name, names)
		ctx.AbortWithStatusJSON(http.StatusNotFound, fmt.Sprintf("name %s not in names %v", name, names))
		return
	}
	conn, err := srv.upgrade(ctx, name, 10*time.Second)
	if err != nil {
		srv.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}
	wsConn := newConnection(conn, name, true)
	if err := srv.connectionManager.addWSConn(wsConn); err != nil {
		srv.logger.Error().Err(err).Msgf("Failed to add connection %s", name)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to add connection"})
		return
	}
	defer srv.connectionManager.closeWSConn(wsConn)

	for {
		var evt = &event.Event{}
		if err := conn.ReadJSON(evt); err != nil {
			if websocket.IsCloseError(errors.Cause(err), websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				srv.logger.Debug().Err(err).Msg("connection closed by client")
			} else {
				srv.logger.Error().Err(err).Msg("Failed to read message")
			}
			break
		}
		//event.Source = secureName
		srv.logger.Debug().Msgf("Received event: %s", evt)
		switch evt.Type {
		case event.TypeNTPQuery:
			srv.logger.Debug().Msgf("Received NTP query from %s: %s", evt.GetSource(), evt.Data)
			if name != evt.GetSource() {
				srv.logger.Error().Msgf("ntp event for %s on %s not allowed", evt.GetSource(), name)
				continue
			}
			data, err := evt.GetData()
			if err != nil {
				srv.logger.Error().Err(err).Msg("Failed to get raw ntp data")
				continue
			}
			raw := data.([]byte)
			result, err := srv.ntpFunc(raw)
			if err != nil {
				srv.logger.Error().Err(err).Msg("Failed to get raw ntp data")
				jsonBytes, _ := json.Marshal(err.Error())
				srv.connectionManager.send(&event.Event{
					Type:   event.TypeNTPError,
					Source: "",
					Target: evt.GetSource(),
					Token:  "",
					Data:   jsonBytes,
				})
				continue
			}
			jsonBytes, _ := json.Marshal(result)
			srv.logger.Debug().Msgf("Sending ntp response to %s: %s - %v", evt.GetSource(), string(jsonBytes), result)
			if err := srv.connectionManager.send(&event.Event{
				Type:   event.TypeNTPResponse,
				Source: "",
				Target: evt.GetSource(),
				Token:  "",
				Data:   jsonBytes,
			}); err != nil {
				srv.logger.Error().Err(err).Msg("Failed to send NTP response")
			}
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
			srv.connectionManager.AddToGroup(name, group)
		case event.TypeDetach:
			if name != evt.GetSource() {
				srv.logger.Error().Msgf("Detach event for %s on %s not allowed", evt.GetSource(), name)
				continue
			}
			data, err := evt.GetData()
			if err != nil {
				srv.logger.Error().Err(err).Msg("Failed to get data for detach event")
				continue
			}
			group := data.(string)
			srv.connectionManager.RemoveFromGroup(name, group)
		default:
			if err := srv.connectionManager.send(evt); err != nil {
				srv.logger.Error().Err(err).Msg("Failed to send event")
			}
		}
		/*
			eventStruct, err := evt.Decode()
			if err != nil {
				srv.logger.Error().Err(err).Msgf("Failed to decode event %s: %s", evt.Type, string(evt.Data))
			}
			srv.logger.Warn().Msgf("Received event [%s] with no target", eventStruct)

		*/
	}
}
