package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"fmt"

	ttyServer "github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type ttyShareClient struct {
	url         string
	connection  *websocket.Conn
	detachKeys string
}

func newTtyShareClient(url string, detachKeys string) *ttyShareClient {
	return &ttyShareClient{
		url:         url,
		connection:  nil,
		detachKeys: detachKeys,
	}
}

func clearScreen() {
	fmt.Fprintf(os.Stdout, "\033[H\033[2J")
}

type wsTextWriter struct {
	conn *websocket.Conn
}


func (w *wsTextWriter) Write(data []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.TextMessage, data)
	return len(data), err
}

type keyListener struct {
	wrappedReader io.Reader
}

func (kl *keyListener) Read(data []byte) (n int, err error) {
	n, err = kl.wrappedReader.Read(data)
	if _, ok := err.(term.EscapeError); ok {
		log.Debug("Escape code detected.")
	}
	return
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

	detachBytes, err := term.ToBytes(c.detachKeys)
	if err != nil {
		log.Errorf("Invalid dettaching keys: %s", c.detachKeys)
		return
	}

	state, err := terminal.MakeRaw(0)
	defer terminal.Restore(0, state)

	c.connection = conn
	// Clear the screen before processing any incoming data
	clearScreen()

	readLoop := func() {
		for {
			var msg ttyServer.MsgAll
			_, r, err := conn.NextReader()
			if err != nil {
				log.Debugf("Connection closed")
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
		kl := &keyListener{
			wrappedReader: term.NewEscapeProxy(os.Stdin, detachBytes),
		}
		_, err := io.Copy(ttyServer.NewTTYProtocolWriter(ww), kl)

		if err != nil {
			log.Debugf("Connection closed")
			c.Stop()
			return
		}
	}
	go writeLoop()
	readLoop()
	clearScreen()
	return
}

func (c *ttyShareClient) Stop() {
	c.connection.Close()
}
