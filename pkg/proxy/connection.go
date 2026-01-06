package proxy

import (
	"emperror.dev/errors"
	"github.com/gorilla/websocket"
)

func newConnection(conn *websocket.Conn, name string, secure bool) *connection {
	return &connection{
		Secure: secure,
		Conn:   conn,
		Name:   name,
	}
}

type connection struct {
	Secure bool
	Conn   *websocket.Conn
	Name   string
}

func (c *connection) Close() error {
	if c.Conn != nil {
		return errors.WithStack(c.Conn.Close())
	}
	return nil
}
