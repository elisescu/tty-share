package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"
)

type ttyShareClient struct {
	url        string
	wsConn     *websocket.Conn
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
		wsConn:     nil,
		detachKeys: detachKeys,
		wcChan:     make(chan os.Signal, 1),
		writeFlag:  1,
	}
}

func clearScreen() {
	fmt.Fprintf(os.Stdout, "\033[H\033[2J")
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
		messageFormat := "\n\rYour window is smaller than the remote window. Please resize.\n\r\tRemote window: %dx%d \n\r\tYour window:   %dx%d \n\r"
		fmt.Printf(messageFormat, c.winSizes.remoteW, c.winSizes.remoteH, c.winSizes.thisW, c.winSizes.thisH)
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

	c.wsConn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
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
	clearScreen()

	protoWS := server.NewTTYProtocolWSLocked(c.wsConn)

	monitorWinChanges := func() {
		// start monitoring the size of the terminal
		signal.Notify(c.wcChan, syscall.SIGWINCH)

		for {
			select {
			case <-c.wcChan:
				c.updateThisWinSize()
				c.updateAndDecideStdoutMuted()
				protoWS.SetWinSize(int(c.winSizes.thisW), int(c.winSizes.thisH))
			}
		}
	}

	readLoop := func() {

		var err error
		for {
			err = protoWS.ReadAndHandle(
				// onWrite
				func(data []byte) {
					if atomic.LoadUint32(&c.writeFlag) != 0 {
						os.Stdout.Write(data)
					}
				},
				// onWindowSize
				func(cols, rows int) {
					c.winSizesMutex.Lock()
					c.winSizes.remoteW = uint16(cols)
					c.winSizes.remoteH = uint16(rows)
					c.winSizesMutex.Unlock()
					c.updateThisWinSize()
					c.updateAndDecideStdoutMuted()
				},
			)

			if err != nil {
				log.Errorf("Error parsing remote message: %s", err.Error())
				if err == io.EOF {
					// Remote WS connection closed
					return
				}
			}
		}
	}

	writeLoop := func() {
		kl := &keyListener{
			wrappedReader: term.NewEscapeProxy(os.Stdin, detachBytes),
		}
		_, err := io.Copy(protoWS, kl)

		if err != nil {
			log.Debugf("Connection closed: %s", err.Error())
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
	c.wsConn.Close()
	signal.Stop(c.wcChan)
}
