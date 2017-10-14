package common

import (
	"fmt"
	"net"
	"time"
)

type wConn struct {
	conn  net.Conn
	debug bool
}

func NewWrappedConn(conn net.Conn, debug bool) net.Conn {
	return &wConn{
		conn:  conn,
		debug: debug,
	}
}

func (c *wConn) Read(b []byte) (n int, err error) {
	n, err = c.conn.Read(b)
	if c.debug {
		fmt.Printf("%s.Read: <%s>, err %s\n", c.conn.LocalAddr().String(), string(b), err)
	}
	return n, err
}

func (c *wConn) Write(b []byte) (n int, err error) {
	n, err = c.conn.Write(b)
	if c.debug {
		fmt.Printf("%s.Wrote: <%s>, err %s\n", c.conn.LocalAddr().String(), string(b), err)
	}
	return
}

func (c *wConn) Close() error {
	return c.conn.Close()
}

func (c *wConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *wConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *wConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *wConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *wConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
