package audioplayer

import (
	"net/url"

	"emperror.dev/errors"
	"github.com/chromedp/chromedp"
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
)

func NewPlayer(name string, browser *browser.Browser, comm *client.Communication, logger zLogger.ZLogger) *Player {
	p := &Player{
		browser: browser,
		comm:    comm,
		logger:  logger,
	}
	comm.On(p.event)
	if err := browser.Run(); err != nil {
		logger.Error().Err(err).Msg("Error starting browser")
	}
	return p
}

type Player struct {
	browser *browser.Browser
	comm    *client.Communication
	status  string
	logger  zLogger.ZLogger
}

func (player *Player) event(evt *event.Event) {
	player.logger.Debug().Str("type", string(evt.GetType())).Str("source", evt.GetSource()).Str("target", evt.GetTarget()).RawJSON("msg", evt.Data).Msg("event")
	switch evt.GetType() {
	case event.TypeBrowserNavigate:
		urlString, err := evt.GetData()
		if err != nil {
			player.logger.Error().Err(err).Msg("Error getting browser navigate data")
			return
		}
		u, err := url.Parse(urlString.(string))
		if err != nil {
			player.logger.Error().Err(err).Msg("Error parsing browser navigate data")
			return
		}
		if err := player.Navigate(u, u.String()); err != nil {
			player.logger.Error().Err(err).Msgf("Error navigating to %s", u.String())
			return
		}
	}
}

func (player *Player) Navigate(u *url.URL, nextStatus string) error {
	if !player.browser.IsRunning() {
		if err := player.browser.Startup(); err != nil {
			return errors.Wrap(err, "could not start browser")
		}
	}

	tasks := chromedp.Tasks{
		chromedp.Navigate(u.String()),
		//		browser.MouseClickXYAction(2,2),
	}
	if err := player.browser.Tasks(tasks); err != nil {
		return errors.Wrapf(err, "could not navigate to %s", u.String())
	}
	player.status = nextStatus
	return nil
}
