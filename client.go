package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/elisescu/tty-share/server"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/moby/term"
	log "github.com/sirupsen/logrus"
)

type ttyShareClient struct {
	url             string
	ttyWsConn       *websocket.Conn
	tunnelWsConn    *websocket.Conn
	tunnelAddresses *string
	detachKeys      string
	wcChan          chan os.Signal
	ioFlagAtomic    uint32 // used with atomic
	winSizes        struct {
		thisW   uint16
		thisH   uint16
		remoteW uint16
		remoteH uint16
	}
	winSizesMutex    sync.Mutex
	tunnelMuxSession *yamux.Session
}

func newTtyShareClient(url string, detachKeys string, tunnelConfig *string) *ttyShareClient {
	return &ttyShareClient{
		url:             url,
		ttyWsConn:       nil,
		detachKeys:      detachKeys,
		wcChan:          make(chan os.Signal, 1),
		ioFlagAtomic:    1,
		tunnelAddresses: tunnelConfig,
	}
}

func clearScreen() {
	fmt.Fprintf(os.Stdout, "\033[H\033[2J")
}

type keyListener struct {
	wrappedReader io.Reader
	ioFlagAtomicP *uint32
}

func (kl *keyListener) Read(data []byte) (n int, err error) {
	n, err = kl.wrappedReader.Read(data)
	if _, ok := err.(term.EscapeError); ok {
		log.Debug("Escape code detected.")
	}

	// If we are not supposed to do any IO, then return 0 bytes read. This happens the local
	// window is smaller than the remote one
	if atomic.LoadUint32(kl.ioFlagAtomicP) == 0 {
		return 0, err
	}

	return
}

func (c *ttyShareClient) updateAndDecideStdoutMuted() {
	log.Infof("This window: %dx%d. Remote window: %dx%d", c.winSizes.thisW, c.winSizes.thisH, c.winSizes.remoteW, c.winSizes.remoteH)

	if c.winSizes.thisH < c.winSizes.remoteH || c.winSizes.thisW < c.winSizes.remoteW {
		atomic.StoreUint32(&c.ioFlagAtomic, 0)
		clearScreen()
		messageFormat := "\n\rYour window is smaller than the remote window. Please resize or press <C-o C-c> to detach.\n\r\tRemote window: %dx%d \n\r\tYour window:   %dx%d \n\r"
		fmt.Printf(messageFormat, c.winSizes.remoteW, c.winSizes.remoteH, c.winSizes.thisW, c.winSizes.thisH)
	} else {
		if atomic.LoadUint32(&c.ioFlagAtomic) == 0 { // clear the screen when changing back to "write"
			// TODO: notify the remote side to "refresh" the content.
			clearScreen()
		}
		atomic.StoreUint32(&c.ioFlagAtomic, 1)
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
	ttyWsPath := resp.Header.Get("TTYSHARE-TTY-WSPATH")
	ttyWSProtocol := resp.Header.Get("TTYSHARE-VERSION")

	ttyTunnelPath := resp.Header.Get("TTYSHARE-TUNNEL-WSPATH")

	// Build the WS URL from the host part of the given http URL and the wsPath
	httpURL, err := url.Parse(c.url)
	if err != nil {
		return
	}
	wsScheme := "ws"
	if httpURL.Scheme == "https" {
		wsScheme = "wss"
	}
	ttyWsURL := wsScheme + "://" + httpURL.Host + ttyWsPath
	ttyTunnelURL := wsScheme + "://" + httpURL.Host + ttyTunnelPath

	log.Debugf("Built the WS URL from the headers: %s", ttyWsURL)

	c.ttyWsConn, _, err = websocket.DefaultDialer.Dial(ttyWsURL, nil)
	if err != nil {
		return
	}
	defer c.ttyWsConn.Close()

	tunnelFunc := func() {
		if *c.tunnelAddresses == "" {
			// Don't build a tunnel
			return
		}

		if ver, err := strconv.Atoi(ttyWSProtocol); err != nil || ver < 2 {
			log.Fatalf("Cannot create a tunnel. Server too old (protocol %d, required min. 2)", ver)
		}

		c.tunnelWsConn, _, err = websocket.DefaultDialer.Dial(ttyTunnelURL, nil)
		if err != nil {
			log.Errorf("Cannot create a tunnel connection with the server. Server needs to allow that")
			return
		}
		defer c.tunnelWsConn.Close()

		a := strings.Split(*c.tunnelAddresses, ":")
		tunnelRemoteAddress := fmt.Sprintf("%s:%s", a[1], a[2])
		tunnelLocalAddress := fmt.Sprintf(":%s", a[0])

		initMsg := server.TunInitMsg{
			Address: tunnelRemoteAddress,
		}

		data, err := json.Marshal(initMsg)
		if err != nil {
			log.Errorf("Could not marshal the tunnel init message: %s", err.Error())
			return
		}

		err = c.tunnelWsConn.WriteMessage(websocket.TextMessage, data)

		if err != nil {
			log.Errorf("Could not initiate the tunnel: %s", err.Error())
			return
		}

		wsWRC := server.WSConnReadWriteCloser{
			WsConn: c.tunnelWsConn,
		}

		localListener, err := net.Listen("tcp", tunnelLocalAddress)
		if err != nil {
			log.Errorf("Could not listen locally for the tunnel: %s", err.Error())

		}

		c.tunnelMuxSession, err = yamux.Server(&wsWRC, nil)
		if err != nil {
			log.Errorf("Could not create mux server: %s", err.Error())
		}

		for {
			localTunconn, err := localListener.Accept()

			if err != nil {
				log.Warnf("Cannot accept local tunnel connections: %s", err.Error())
				return
			}

			muxClient, err := c.tunnelMuxSession.Open()
			if err != nil {
				log.Warnf("Cannot create a muxer to the remote, over ws: %s", err.Error())
				return
			}

			go func() {
				io.Copy(muxClient, localTunconn)
				defer localTunconn.Close()
				defer muxClient.Close()
			}()

			go func() {
				io.Copy(localTunconn, muxClient)
				defer localTunconn.Close()
				defer muxClient.Close()
			}()

		}
	}

	detachBytes, err := term.ToBytes(c.detachKeys)
	if err != nil {
		log.Errorf("Invalid dettaching keys: %s", c.detachKeys)
		return
	}

	state, err := term.MakeRaw(os.Stdin.Fd())
	defer term.RestoreTerminal(os.Stdin.Fd(), state)
	clearScreen()

	protoWS := server.NewTTYProtocolWSLocked(c.ttyWsConn)

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
					if atomic.LoadUint32(&c.ioFlagAtomic) != 0 {
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
			ioFlagAtomicP: &c.ioFlagAtomic,
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
	go tunnelFunc()
	readLoop()

	clearScreen()
	return
}

func (c *ttyShareClient) Stop() {
	// if we had a tunnel, close it
	if c.tunnelMuxSession != nil {
		c.tunnelMuxSession.Close()
		c.tunnelWsConn.Close()
	}
	c.ttyWsConn.Close()
	signal.Stop(c.wcChan)
}
