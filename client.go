package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"

	ttyServer "github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type ttyShareClient struct {
	url        string
	connection *websocket.Conn
}

func newTtyShareClient(url string) *ttyShareClient {
	return &ttyShareClient{
		url:        url,
		connection: nil,
	}
}

type wsTextWriter struct {
	conn *websocket.Conn
}

func (w *wsTextWriter) Write(data []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.TextMessage, data)
	return len(data), err
}

func (c *ttyShareClient) Run() (err error) {
	log.Debugf("Connecting as a client to %s ..", c.url)

	resp, err := http.Get(c.url)

	if err != nil {
		return
	}

	// Get the path of the websockts route from the header
	wsPath := resp.Header.Get("TTYSHARE-WSPATH")

	// Build the WS URL from the host part of the given http URL and the wsPath
	httpURL, err := url.Parse(c.url)
	if err != nil {
		return
	}
	wsScheme := "ws"
	if httpURL.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := wsScheme + "://" + httpURL.Host + wsPath

	log.Debugf("Built the WS URL from the headers: %s", wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return
	}

	state, err := terminal.MakeRaw(0)
	defer terminal.Restore(0, state)

	c.connection = conn
	readLoop := func() {
		for {
			var msg ttyServer.MsgAll
			_, r, err := conn.NextReader()
			if err != nil {
				log.Debugf("Connection closed\n")
				return
			}
			err = json.NewDecoder(r).Decode(&msg)
			if err != nil {
				log.Errorf("Cannot read JSON: %s", err.Error())
			}

			switch msg.Type {
			case ttyServer.MsgIDWrite:
				var msgWrite ttyServer.MsgTTYWrite
				err := json.Unmarshal(msg.Data, &msgWrite)

				if err != nil {
					log.Errorf("Cannot read JSON: %s", err.Error())
				}

				os.Stdout.Write(msgWrite.Data)
			case ttyServer.MsgIDWinSize:
				log.Infof("Remote window changed its size")
				// We ignore the window size changes - can't do much about that for
				// now.

				// TODO: Maybe just clear the screen, and display an error message
				// if the remote window gets bigger than this terminal window - when
				// it does, it usually messes up the output
			}
		}
	}

	writeLoop := func() {
		ww := &wsTextWriter{
			conn: conn,
		}
		_, err := io.Copy(ttyServer.NewTTYProtocolWriter(ww), os.Stdin)

		if err != nil {
			log.Debugf("Connection closed.\n")
			return
		}
	}
	go writeLoop()
	readLoop()
	return
}

func (c *ttyShareClient) Stop() {
	c.connection.Close()
}
