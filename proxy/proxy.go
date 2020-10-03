package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"

	"github.com/hashicorp/yamux"
	log "github.com/sirupsen/logrus"
)

type HelloClient struct {
	Version string
	Data    string
}

type HelloServer struct {
	Version   string
	SessionID string
	PublicURL string
	Data      string
}

type proxyConnection struct {
	muxSession      *yamux.Session
	backConnAddress string
	SessionID       string
	PublicURL       string
}

func NewProxyConnection(backConnAddrr, proxyAddr string, noTLS bool) (*proxyConnection, error) {
	var conn net.Conn
	var err error

	if noTLS {
		conn, err = net.Dial("tcp", proxyAddr)
		if err != nil {
			return nil, err
		}
	} else {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		conn, err = tls.Dial("tcp", proxyAddr, &tls.Config{RootCAs: roots})
		if err != nil {
			return nil, err
		}
	}

	// C -> S: HelloCLient
	// S -> C: HelloServer {sesionID}
	je := json.NewEncoder(conn)
	// TODO: extract these strings constants somewhere at some point
	helloC := HelloClient{
		Version: "1",
		Data:    "-",
	}
	err = je.Encode(helloC)
	if err != nil {
		return nil, err
	}

	jd := json.NewDecoder(conn)
	var helloS HelloServer
	err = jd.Decode(&helloS)
	if err != nil {
		return nil, err
	}

	log.Debugf("Connected to %s tty-proxy: version=%s, sessionID=%s", helloS.PublicURL, helloS.Version, helloS.SessionID)
	session, err := yamux.Server(conn, nil)

	return &proxyConnection{
		muxSession:      session,
		backConnAddress: backConnAddrr,
		SessionID:       helloS.SessionID,
		PublicURL:       helloS.PublicURL,
	}, nil
}

func (p *proxyConnection) RunProxy() {
	for {
		frontConn, err := p.muxSession.Accept()
		if err != nil {
			log.Debugf("tty-proxy connection closed: %s", err.Error())
			return
		}
		defer frontConn.Close()

		go func() {
			backConn, err := net.Dial("tcp", p.backConnAddress)

			if err != nil {
				log.Errorf("Cannot proxy the connection to the target HTTP server: %s", err.Error())
				return
			}
			defer backConn.Close()

			pipeConnectionsAndWait(backConn, frontConn)
		}()
	}
}

func (p *proxyConnection) Stop() {
	p.muxSession.Close()
}

func errToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return "nil"
}

func pipeConnectionsAndWait(backConn, frontConn net.Conn) error {
	errChan := make(chan error, 2)

	backConnAddr := backConn.RemoteAddr().String()
	frontConnAddr := frontConn.RemoteAddr().String()

	log.Debugf("Piping the two conn %s <-> %s ..", backConnAddr, frontConnAddr)

	copyAndNotify := func(dst, src net.Conn, info string) {
		n, err := io.Copy(dst, src)
		log.Debugf("%s: piping done with %d bytes, and err %s", info, n, errToString(err))
		errChan <- err

		// Close both connections when done with copying. Yeah, both will beclosed two
		// times, but it doesn't matter. By closing them both, we unblock the other copy
		// call which would block indefinitely otherwise
		dst.Close()
		src.Close()
	}

	go copyAndNotify(backConn, frontConn, "front->back")
	go copyAndNotify(frontConn, backConn, "back->front")
	err1 := <-errChan
	err2 := <-errChan

	log.Debugf("Piping finished for %s <-> %s .", backConnAddr, frontConnAddr)

	// Return one of the two error that is not nil
	if err1 != nil {
		return err1
	}
	return err2
}
