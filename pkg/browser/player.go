package browser

import (
	"time"

	"github.com/je4/securedisplay/pkg/player"
)

func NewPlayer(name string, browser *Browser) (*Player, error) {
	player := &Player{
		browser: browser,
		name:    name,
	}
	return player, nil
}

type Player struct {
	browser *Browser
	name    string
}

func (p *Player) Load(urn string) error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Play() error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Pause() error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Resume() error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) SeekTime(pos time.Duration) error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) SeekElement(element int) error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Unload() error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) IsRunning() bool {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Close() error {
	//TODO implement me
	panic("implement me")
}

func (p *Player) Init() error {
	//TODO implement me
	panic("implement me")
}

var _ player.Player = (*Player)(nil)
