package client

import (
	"net"
	"time"
)

type ntpConn struct {
}

func (conn *ntpConn) Write(b []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) Close() error {
	//TODO implement me
	panic("implement me")
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
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) SetReadDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) SetWriteDeadline(t time.Time) error {
	//TODO implement me
	panic("implement me")
}

func (conn *ntpConn) Read(b []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

var _ net.Conn = (*ntpConn)(nil)
