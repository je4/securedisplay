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

func (c *Communication) Start() error {
	go func() {
		c.wg.Add(1)
		defer func() {
			c.logger.Info().Msgf("closing connection: %s", c.name)
			if err := c.proxy.Close(); err != nil {
				c.logger.Error().Err(err).Msgf("cannot close connection: %s", c.name)
			}
			c.wg.Done()
		}()
		for {
			evt, err := c.Receive()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					c.logger.Debug().Err(err).Msgf("connection closed: %s", c.name)
					return
				}
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
					c.logger.Debug().Err(err).Msgf("unexpected close error: %s", c.name)
					return
				}
				c.logger.Error().Err(err).Msgf("cannot read event: %s", c.name)
			}
			if c.recFunc != nil {
				data, err := evt.Decode()
				if err != nil {
					c.logger.Error().Err(err).Msgf("cannot decode event: %s", c.name)
					continue
				}
				c.recFunc(data, evt.Target, evt.Token)
			} else {
				c.logger.Debug().Msgf("no receiver function set for event: %s", c.name)
			}
		}
	}()
	return nil
}

func (c *Communication) Stop() error {
	deadline := time.Now().Add(10 * time.Second)
	err := c.proxy.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline,
	)
	if err != nil {
		return errors.Wrapf(err, "cannot send close message: %s", c.name)
	}
	closeChan := make(chan struct{})
	go func() {
		defer close(closeChan)
		c.wg.Wait()
	}()
	select {
	case <-closeChan:
	case <-time.After(time.Second * 10):
		c.logger.Warn().Msgf("timeout waiting for connection to close: %s", c.name)
		if err := c.proxy.Close(); err != nil {
			return errors.Wrapf(err, "cannot close connection: %s", c.name)
		}
	}
	return nil
}

func (c *Communication) On(recFunc recFuncType) {
	c.recFunc = recFunc
}

func (c *Communication) Receive() (*Event, error) {
	var evt Event
	if err := c.proxy.ReadJSON(&evt); err != nil {
		return nil, errors.Wrapf(err, "cannot read event")
	}
	return &evt, nil
}

func (c *Communication) Send(data DataInterface, target, token string) error {
	evt, err := NewEvent(data, target, token)
	if err != nil {
		return errors.Wrapf(err, "cannot create event: %v", data)
	}
	if err = errors.Wrapf(c.proxy.WriteJSON(evt), "cannot send event: %v", evt); err != nil {
		return errors.Wrapf(err, "cannot send event: %v", evt)
	}
	return nil
}
