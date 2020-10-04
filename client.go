package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	ttyServer "github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"
)

type ttyShareClient struct {
	url        string
	connection *websocket.Conn
	detachKeys string
	wcChan     chan os.Signal
	writeFlag  uint32 // used with atomic
	winSizes   struct {
		thisW   uint16
		thisH   uint16
		remoteW uint16
		remoteH uint16
	}
	winSizesMutex sync.Mutex
}

func newTtyShareClient(url string, detachKeys string) *ttyShareClient {
	return &ttyShareClient{
		url:        url,
		connection: nil,
		detachKeys: detachKeys,
		wcChan:     make(chan os.Signal, 1),
		writeFlag:  1,
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

func (c *ttyShareClient) updateAndDecideStdoutMuted() {
	log.Infof("This window: %dx%d. Remote window: %dx%d", c.winSizes.thisW, c.winSizes.thisH, c.winSizes.remoteW, c.winSizes.remoteH)

	if c.winSizes.thisH < c.winSizes.remoteH || c.winSizes.thisW < c.winSizes.remoteW {
		atomic.StoreUint32(&c.writeFlag, 0)
		clearScreen()
		fmt.Printf("\r\n\nYour terminal window has to be bigger than %dx%d\r\nDetach with <%s>, resize your window, and reconect.\r\n",
			c.winSizes.remoteW, c.winSizes.remoteH, c.detachKeys)
	} else {
		if atomic.LoadUint32(&c.writeFlag) == 0 { // clear the screen when changing back to "write"
			// TODO: notify the remote side to "refresh" the content.
			clearScreen()
		}
		atomic.StoreUint32(&c.writeFlag, 1)
	}
}

func (c *ttyShareClient) updateThisWinSize() {
	size, err := term.GetWinsize(os.Stdin.Fd())
	if err == nil {
		c.winSizesMutex.Lock()
		c.winSizes.thisW = size.Width
		c.winSizes.thisH = size.Height
		c.winSizesMutex.Unlock()
	}
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

	state, err := term.MakeRaw(os.Stdin.Fd())
	defer term.RestoreTerminal(os.Stdin.Fd(), state)

	c.connection = conn

	clearScreen()
	// start monitoring the size of the terminal
	signal.Notify(c.wcChan, syscall.SIGWINCH)
	defer signal.Stop(c.wcChan)

	monitorWinChanges := func() {
		for {
			select {
			case <-c.wcChan:
				log.Debugf("Detected new win size")
				c.updateThisWinSize()
				c.updateAndDecideStdoutMuted()
			}
		}
	}

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

				if atomic.LoadUint32(&c.writeFlag) != 0 {
					os.Stdout.Write(msgWrite.Data)
				}
			case ttyServer.MsgIDWinSize:
				var msgRemoteWinSize ttyServer.MsgTTYWinSize
				err := json.Unmarshal(msg.Data, &msgRemoteWinSize)
				if err != nil {
					continue
				}
				c.winSizesMutex.Lock()
				c.winSizes.remoteW = uint16(msgRemoteWinSize.Cols)
				c.winSizes.remoteH = uint16(msgRemoteWinSize.Rows)
				c.winSizesMutex.Unlock()
				c.updateThisWinSize()
				c.updateAndDecideStdoutMuted()
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

	go monitorWinChanges()
	go writeLoop()
	readLoop()

	clearScreen()
	return
}

func (c *ttyShareClient) Stop() {
	c.connection.Close()
}
