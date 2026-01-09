package genericplayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/chromedp/chromedp"
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
)

func NewPlayer(ctx context.Context, u *url.URL, browser *browser.Browser, comm *client.Communication, logger zLogger.ZLogger) *Player {
	p := &Player{
		browser: browser,
		comm:    comm,
		logger:  logger,
		url:     u,
		ctx:     ctx,
	}
	comm.On(p.event)
	if err := browser.Run(); err != nil {
		logger.Error().Err(err).Msg("Error starting browser")
	}
	if err := browser.Navigate(u); err != nil {
		logger.Error().Err(err).Msgf("Error navigating to %s", u.String())
	}
	return p
}

type Player struct {
	browser *browser.Browser
	comm    *client.Communication
	status  string
	logger  zLogger.ZLogger
	url     *url.URL
	ctx     context.Context
}

func (player *Player) event(evt *event.Event) {
	player.logger.Debug().Str("type", string(evt.GetType())).Str("source", evt.GetSource()).Str("target", evt.GetTarget()).RawJSON("msg", evt.Data).Msg("event")
	switch evt.GetType() {
	default:
		eventBytes, err := json.Marshal(evt.Data)
		if err != nil {
			player.logger.Error().Err(err).Msg("Error marshalling data")
			return
		}
		var res string
		player.browser.Tasks(
			chromedp.Tasks{
				chromedp.Evaluate(fmt.Sprintf("eval(\"%s\")", eventBytes), &res),
			},
		)
	}
}
