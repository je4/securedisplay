package client

import (
	"encoding/json"
	"net"
	"time"

	"emperror.dev/errors"
	"github.com/je4/securedisplay/pkg/event"
)

func newNTPConn(comm *Communication) net.Conn {
	conn := &ntpConn{
		comm: comm,
		ch:   make(chan []byte),
	}
	comm.SetNTPReceiver(conn.ch)
	return conn
}

type ntpConn struct {
	comm         *Communication
	ch           chan []byte
	deadline     time.Time
	readDeadline time.Time
}

func (conn *ntpConn) Write(b []byte) (n int, err error) {
	jsonBytes, err := json.Marshal(b)
	if err != nil {
		return 0, errors.Wrapf(err, "error marshalling %s", conn.comm.name)
	}
	conn.comm.logger.Debug().Msgf("Sending ntp query to %s: %s", conn.comm.name, string(jsonBytes))
	if err := conn.comm.Send(&event.Event{
		Type:   event.TypeNTPQuery,
		Source: conn.comm.name,
		Target: "",
		Token:  "",
		Data:   jsonBytes,
	}); err != nil {
		return 0, errors.Wrapf(err, "cannot send ntp-query event to %s: %v", conn.comm.name, b)
	}
	return len(b), nil
}

func (conn *ntpConn) Close() error {
	conn.comm.RemoveNTPReceiver()
	//close(conn.ch)
	return nil
}

func (conn *ntpConn) LocalAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) RemoteAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) SetDeadline(t time.Time) error {
	conn.deadline = t
	return nil
}

func (conn *ntpConn) SetReadDeadline(t time.Time) error {
	conn.readDeadline = t
	return nil
}

func (conn *ntpConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) Read(b []byte) (n int, err error) {
	var deadline time.Time
	if !conn.readDeadline.IsZero() {
		deadline = conn.readDeadline
	} else {
		deadline = conn.deadline
	}
	if deadline.IsZero() {
		b = <-conn.ch
		//conn.comm.logger.Debug().Msgf("NTP read from %s: %v", conn.comm.name, b)
		return len(b), nil
	}
	select {
	case ret := <-conn.ch:
		copy(b, ret)
		//conn.comm.logger.Debug().Msgf("NTP read from %s: %v", conn.comm.name, b)
		return len(ret), nil
	case <-time.After(time.Until(deadline)):
		conn.comm.logger.Error().Msgf("NTP timeout from %s", conn.comm.name)
		return 0, errors.New("timed out")
	}
}

var _ net.Conn = (*ntpConn)(nil)
