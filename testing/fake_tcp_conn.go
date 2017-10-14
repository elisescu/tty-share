package testing

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type fakeWriteCb func(io.Writer, []byte) (int, error)

// Use this carefully.  Not thread safe
type fakeTCPConn struct {
	readPipe  *io.PipeReader
	writePipe *io.PipeWriter
	debug     bool
	writeCb   fakeWriteCb
	deadline  time.Time
}

func NewFakeTCPConn(debug bool, writeCb fakeWriteCb) *fakeTCPConn {
	ret := &fakeTCPConn{debug: debug}
	ret.readPipe, ret.writePipe = io.Pipe()
	ret.writeCb = writeCb
	return ret
}

func (conn *fakeTCPConn) Write(b []byte) (int, error) {
	if conn.debug {
		fmt.Printf("fakeTCP.Write: %s\n", string(b))
	}

	if conn.writeCb != nil {
		return conn.writeCb(conn.writePipe, b)
	}
	return conn.writePipe.Write(b)
}

// If Read times out, the connection can't be used anymore.
// TODO: maybe fix that
func (conn *fakeTCPConn) Read(b []byte) (int, error) {
	c := make(chan int)
	n := 0
	err := error(nil)

	doRead := func() {
		n, err = conn.readPipe.Read(b)

		if conn.debug {
			fmt.Printf("fakeTCP.Read: %s\n", string(b))
		}
	}

	// If we have no deadline, then let Read wait forever
	var zeroTime time.Time
	if conn.deadline == zeroTime {
		doRead()
		return n, err
	}

	// Otherwise, do the read in a go routine
	go func() {
		doRead()
		close(c)
	}()

	select {
	case <-c:
		return n, err
	case <-time.After(conn.deadline.Sub(time.Now())):
		// TODO: we timed out. What to do? Close the pipe?
		conn.writePipe.CloseWithError(errors.New("timeout"))
		conn.readPipe.CloseWithError(errors.New("timeout"))
		// don't return here - closing with error, will make the readPipe.Read return
		// the above error, passed to ClosedWithError
	}

	return n, err
}

func (conn *fakeTCPConn) Close() (err error) {
	err = conn.writePipe.Close()
	if err != nil {
		return
	}
	err = conn.readPipe.Close()
	if err != nil {
		return
	}
	return
}

func (conn *fakeTCPConn) LocalAddr() net.Addr {
	panic("LocalAddr not implemented")
	return nil
}

func (conn *fakeTCPConn) RemoteAddr() net.Addr {
	panic("RemoteAddr not implemented")
	return nil
}

func (conn *fakeTCPConn) SetDeadline(t time.Time) error {
	conn.deadline = t
	return nil
}

func (conn *fakeTCPConn) SetReadDeadline(t time.Time) error {
	panic("SetReadDeadline not implemented")
	return nil
}

func (conn *fakeTCPConn) SetWriteDeadline(t time.Time) error {
	panic("SetWriteDeadline not implemented")
	return nil
}
