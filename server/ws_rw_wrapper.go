package server

import (
	"io"

	"github.com/gorilla/websocket"
)

type TunInitMsg struct {
	Address string
}

type WSConnReadWriteCloser struct {
	WsConn *websocket.Conn
	reader io.Reader
}

func (conn *WSConnReadWriteCloser) Read(p []byte) (n int, err error) {
	// Weird method here, as we need to do a few things:
	//   - re-use the WS reader between different calls of this function. If the existing reader
	//       has no more data, then get another reader (NextReader())
	//   - if we get a CloseAbnormalClosure, or CloseGoingAway error message from WS, we need to
	//       transform that into a io.EOF, otherwise yamux will complain. We use yamux on top of this
	//       reader interface, in order to multiplex multiple streams
	// More here:
	// https://github.com/hashicorp/yamux/blob/574fd304fd659b0dfdd79e221f4e34f6b7cd9ed2/session.go#L554
	// https://github.com/gorilla/websocket/blob/b65e62901fc1c0d968042419e74789f6af455eb9/examples/chat/client.go#L67
	// https://stackoverflow.com/questions/61108552/go-websocket-error-close-1006-abnormal-closure-unexpected-eof

	filterErr := func() {

		if err != nil && !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			// if we have an error != nil, and it's one of the two, then return EOF
			err = io.EOF
		}
	}

	defer filterErr()

	if conn.reader != nil {
		n, err = conn.reader.Read(p)

		if err == io.EOF {
			// if this reader has no more data, get the next reader
			_, conn.reader, err = conn.WsConn.NextReader()

			if err == nil {
				// and read in this same call as well
				return conn.reader.Read(p)
			}
		}
	} else {
		_, conn.reader, err = conn.WsConn.NextReader()
	}
	return
}

func (conn *WSConnReadWriteCloser) Write(p []byte) (n int, err error) {
	return len(p), conn.WsConn.WriteMessage(websocket.BinaryMessage, p)
}

func (conn *WSConnReadWriteCloser) Close() error {
	return conn.WsConn.Close()
}
