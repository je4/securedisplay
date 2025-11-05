package event

import (
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gorilla/websocket"
	"github.com/je4/utils/v2/pkg/zLogger"
)

type recFuncType func(data DataInterface, sender, token string)

func NewCommunication(proxy *websocket.Conn, name string, logger zLogger.ZLogger) *Communication {
	return &Communication{
		proxy:  proxy,
		name:   name,
		logger: logger,
		wg:     sync.WaitGroup{},
	}
}

type Communication struct {
	proxy   *websocket.Conn
	name    string
	recFunc recFuncType
	logger  zLogger.ZLogger
	wg      sync.WaitGroup
}

func (comm *Communication) Start() error {
	go func() {
		comm.wg.Add(1)
		defer func() {
			comm.logger.Info().Msgf("closing connection: %s", comm.name)
			if err := comm.proxy.Close(); err != nil {
				comm.logger.Error().Err(err).Msgf("cannot close connection: %s", comm.name)
			}
			comm.wg.Done()
		}()
		for {
			evt, err := comm.Receive()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					comm.logger.Debug().Err(err).Msgf("connection closed: %s", comm.name)
					return
				}
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					comm.logger.Debug().Err(err).Msgf("unexpected close error: %s", comm.name)
					return
				}
				comm.logger.Error().Err(err).Msgf("cannot read event: %s", comm.name)
			}
			if comm.recFunc != nil {
				data, err := evt.Decode()
				if err != nil {
					comm.logger.Error().Err(err).Msgf("cannot decode event: %s", comm.name)
					continue
				}
				comm.recFunc(data, evt.Target, evt.Token)
			} else {
				comm.logger.Debug().Msgf("no receiver function set for event: %s", comm.name)
			}
		}
	}()
	return nil
}

func (comm *Communication) Stop() error {
	deadline := time.Now().Add(10 * time.Second)
	err := comm.proxy.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline,
	)
	if err != nil {
		return errors.Wrapf(err, "cannot send close message: %s", comm.name)
	}
	closeChan := make(chan struct{})
	go func() {
		defer close(closeChan)
		comm.wg.Wait()
	}()
	select {
	case <-closeChan:
	case <-time.After(time.Second * 10):
		comm.logger.Warn().Msgf("timeout waiting for connection to close: %s", comm.name)
		if err := comm.proxy.Close(); err != nil {
			return errors.Wrapf(err, "cannot close connection: %s", comm.name)
		}
	}
	return nil
}

func (comm *Communication) On(recFunc recFuncType) {
	comm.recFunc = recFunc
}

func (comm *Communication) Receive() (*Event, error) {
	var evt Event
	if err := comm.proxy.ReadJSON(&evt); err != nil {
		return nil, errors.Wrapf(err, "cannot read event")
	}
	return &evt, nil
}

func (comm *Communication) Send(data DataInterface, target, token string) error {
	evt, err := NewEvent(data, target, token)
	if err != nil {
		return errors.Wrapf(err, "cannot create event: %v", data)
	}
	if err = errors.Wrapf(comm.proxy.WriteJSON(evt), "cannot send event: %v", evt); err != nil {
		return errors.Wrapf(err, "cannot send event: %v", evt)
	}
	return nil
}
