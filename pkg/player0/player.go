package player0

import "time"

type Player interface {
	Init() error
	Load(urn string) error
	Play() error
	Pause() error
	Resume() error
	SeekTime(pos time.Duration) error
	SeekElement(element int) error
	Unload() error
	IsRunning() bool
	Close() error
}
