package audioplayer

import (
	"github.com/je4/securedisplay/pkg/browser"
	"github.com/je4/securedisplay/pkg/client"
	"github.com/je4/securedisplay/pkg/event"
)

func NewPlayer(name string, browser *browser.Browser, comm *client.Communication) *Player {
	p := &Player{
		browser: browser,
		comm:    comm,
	}
	comm.On(p.event)
	return p
}

type Player struct {
	browser *browser.Browser
	comm    *client.Communication
}

func (player *Player) event(evt *event.Event) {

}
