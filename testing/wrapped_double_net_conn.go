package testing

import (
	"net"
	"sync"

	"github.com/elisescu/tty-share/common"
)

func NewDoubleNetConn(debug bool) (client net.Conn, server net.Conn) {
	var wg sync.WaitGroup
	wg.Add(1)
	var err error

	listener, err := net.Listen("tcp", "localhost:0")
	defer listener.Close()
	if err != nil {
		panic(err.Error())
	}

	go func() {
		server, err = listener.Accept()
		if err != nil {
			panic(err.Error())
		}
		wg.Done()
	}()

	client, err = net.Dial("tcp", listener.Addr().String())

	if err != nil {
		panic(err.Error())
	}

	wg.Wait()
	return common.NewWrappedConn(client, debug), common.NewWrappedConn(server, debug)
}
