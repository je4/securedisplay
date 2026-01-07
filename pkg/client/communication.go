package client

import (
	"net"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/beevik/ntp"
	"github.com/gorilla/websocket"
	"github.com/je4/securedisplay/pkg/event"
	"github.com/je4/utils/v2/pkg/zLogger"
)

type recFuncType func(evt *event.Event)

func NewCommunication(proxy *websocket.Conn, name string, logger zLogger.ZLogger) *Communication {
	return &Communication{
		proxyConn: proxy,
		name:      name,
		logger:    logger,
		wg:        sync.WaitGroup{},
		ntpConn:   make(chan<- []byte),
	}
}

type Communication struct {
	proxyConn *websocket.Conn
	name      string
	recFunc   recFuncType
	logger    zLogger.ZLogger
	wg        sync.WaitGroup
	ntpConn   chan<- []byte
}

func (comm *Communication) SetNTPReceiver(ch chan<- []byte) {
	comm.ntpConn = ch
}

func (comm *Communication) RemoveNTPReceiver() {
	if comm.ntpConn != nil {
		close(comm.ntpConn)
	}
	comm.ntpConn = nil
}

func (comm *Communication) Start() error {
	go func() {
		comm.wg.Add(1)
		defer func() {
			comm.logger.Info().Msgf("closing connection: %s", comm.name)
			if err := comm.proxyConn.Close(); err != nil {
				comm.logger.Error().Err(err).Msgf("cannot close connection: %s", comm.name)
			}
			comm.wg.Done()
		}()
		for {
			evt, err := comm.Receive()
			if err != nil {
				cause := errors.Cause(err)
				if websocket.IsCloseError(cause, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
					comm.logger.Debug().Err(err).Msgf("connection closed: %s", comm.name)
					return
				}
				if websocket.IsUnexpectedCloseError(cause, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
					comm.logger.Debug().Err(err).Msgf("unexpected close error: %s", comm.name)
					return
				}
				comm.logger.Error().Err(err).Msgf("cannot read event: %s", comm.name)
				continue
			}
			comm.logger.Debug().Msgf("received event from %s: %s", evt.GetSource(), evt.Type)
			switch evt.Type {
			case event.TypeNTPResponse, event.TypeNTPError:
				comm.logger.Debug().Msgf("received NTP event %s from %s: %s", evt.GetType(), evt.GetSource(), evt.Data)
				if comm.ntpConn == nil {
					continue
				}
				data, err := evt.GetData()
				if err != nil {
					comm.logger.Error().Err(err).Msgf("cannot read event: %s", comm.name)
					continue
				}
				comm.ntpConn <- data.([]byte)
			default:
				if comm.recFunc != nil {
					comm.recFunc(evt)
				} else {
					comm.logger.Debug().Msgf("no receiver function set for event: %s", comm.name)
				}
			}
		}
	}()
	return nil
}

func (comm *Communication) Stop() error {
	deadline := time.Now().Add(10 * time.Second)
	err := comm.proxyConn.WriteControl(
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
		if err := comm.proxyConn.Close(); err != nil {
			return errors.Wrapf(err, "cannot close connection: %s", comm.name)
		}
	}
	return nil
}

func (comm *Communication) On(recFunc recFuncType) {
	comm.recFunc = recFunc
}

func (comm *Communication) Receive() (*event.Event, error) {
	var evt event.Event
	if err := comm.proxyConn.ReadJSON(&evt); err != nil {
		return nil, errors.Wrapf(err, "cannot read event")
	}
	return &evt, nil
}

func (comm *Communication) Send(evt *event.Event) error {
	evt.Source = comm.name
	/*
		evt, err := event.NewEvent(data, target, token)
		if err != nil {
			return errors.Wrapf(err, "cannot create event: %v", data)
		}
	*/
	if err := errors.Wrapf(comm.proxyConn.WriteJSON(evt), "cannot send event: %v", evt); err != nil {
		return errors.Wrapf(err, "cannot send event: %v", evt)
	}
	return nil
}

func (comm *Communication) NTP() error {
	/*
		resp0, err := ntp.Query("0.beevik-ntp.pool.ntp.org")
		if err != nil {
			return errors.Wrapf(err, "cannot get NTP response")
		}
		comm.logger.Debug().Msgf("NTP0 response: %v", resp0)

	*/

	conn := newNTPConn(comm)
	defer conn.Close()
	options := ntp.QueryOptions{
		Timeout: 30 * time.Second,
		Dialer: func(localAddress, remoteAddress string) (net.Conn, error) {
			return conn, nil
		},
	}
	response, err := ntp.QueryWithOptions("proxy", options)
	if err != nil {
		return errors.Wrapf(err, "cannot send NTP request")
	}
	comm.logger.Info().Msgf("NTP clock offset: %s", response.ClockOffset)
	return nil
}
