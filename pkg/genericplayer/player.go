package genericplayer

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
)

func NewPlayer(ctx context.Context, u *url.URL, browser *browser.Browser, comm *client.Communication, logger zLogger.ZLogger) *Player {
	p := &Player{
		browser:   browser,
		comm:      comm,
		logger:    logger,
		url:       u,
		ctx:       ctx,
		closeChan: make(chan struct{}),
	}
	p.Run()
	return p
}

type Player struct {
	browser   *browser.Browser
	comm      *client.Communication
	status    string
	logger    zLogger.ZLogger
	url       *url.URL
	ctx       context.Context
	closeChan chan struct{}
}

type PlayerStatus struct {
	CurrentTime float64 `json:"currentTime"` // Aktuelle Position in Sekunden
	Duration    float64 `json:"duration"`    // Gesamtlänge in Sekunden
	Muted       bool    `json:"muted"`       // Ton aus/an
	Paused      bool    `json:"paused"`      // Pause-Status
	Status      string  `json:"status"`      // z.B. "play", "stop", "pause"
	SystemTime  int64   `json:"systemTime"`  // Unix-Zeitstempel in Millisekunden
	Volume      float64 `json:"volume"`      // Lautstärke (oft 0.0 bis 1.0)
}

func (player *Player) Run() error {
	player.comm.On(player.event)
	if err := player.browser.Run(); err != nil {
		player.logger.Error().Err(err).Msg("Error starting browser")
	}
	if err := player.browser.Navigate(player.url); err != nil {
		player.logger.Error().Err(err).Msgf("Error navigating to %s", player.url.String())
	}
	go func() {
		for {
			select {
			case <-player.closeChan:
				return
			case <-time.After(1 * time.Second):
				res, err := player.browser.Evaluate("getStatus", "")
				if err != nil {
					player.logger.Error().Err(err).Msg("Error getting status")
					continue
				}
				if res == "" {
					continue
				}
				var obj = &PlayerStatus{}
				if err := json.Unmarshal([]byte(res), obj); err != nil {
					player.logger.Error().Err(err).Msg("Error unmarshalling status")
					continue
				}
				player.logger.Debug().Interface("obj", obj).Msg("Got status")
				obj.SystemTime += player.comm.ClockOffset.Milliseconds()
				jsonData, err := json.Marshal(obj)
				if err != nil {
					player.logger.Error().Err(err).Msg("Error marshalling status")
					continue
				}
				player.comm.Send(&event.Event{
					Type:   "status",
					Source: "",
					Target: "core",
					Token:  "",
					Data:   jsonData,
				})
			}

		}

	}()
	return nil
}

func (player *Player) event(evt *event.Event) {
	player.logger.Debug().Str("type", string(evt.GetType())).Str("source", evt.GetSource()).Str("target", evt.GetTarget()).RawJSON("msg", evt.Data).Msg("event")
	switch evt.GetType() {
	default:
		_, err := player.browser.Evaluate("event", evt)
		if err != nil {
			player.logger.Error().Err(err).Msg("Error evaluating event")
		}

	}
}

func (player *Player) Close() {
	close(player.closeChan)
}
