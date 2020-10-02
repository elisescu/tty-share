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
	// S -> C: HelloServer (sesionID)
	je := json.NewEncoder(conn)
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

	log.Debugf("Got from the ReverseProxy: version=%s, sessionID=%s", helloS.Version, helloS.SessionID)
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
		conn, err := p.muxSession.Accept()
		if err != nil {
			log.Errorf("tty-proxy connection closed.\n")
			return
		}

		go func() {
			dst, err := net.Dial("tcp", p.backConnAddress)
			defer dst.Close()
			defer conn.Close()

			if err != nil {
				log.Errorf("Client: Can't connect to the target HTTP server: %s\n", err.Error())
			}
			glueConnAndWait(dst, conn)
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

func glueConnAndWait(conn1, conn2 net.Conn) error {
	errChan := make(chan error, 2)

	log.Debugf("Starting the glue of the two conn %s  %s", conn1.LocalAddr().String(), conn2.LocalAddr().String())

	copyAndNotify := func(dst, src net.Conn) {
		n, err := io.Copy(dst, src)
		log.Debugf("Wrote %d bytes,  %s -> %s\n", n, src.LocalAddr().String(), dst.LocalAddr().String())
		if err != nil {
			log.Debugf("  -- ended with error: %s\n", err.Error())
		}
		errChan <- err
	}

	go copyAndNotify(conn1, conn2)
	go copyAndNotify(conn2, conn1)
	err1 := <-errChan
	err2 := <-errChan

	log.Debugf("Finished the glued connections with: %s and %s", errToString(err1), errToString(err2))
	return err1
}
