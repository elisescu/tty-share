package testing

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/elisescu/tty-share/common"
)

// Returns true if waiting for the wg timed out
func wgWaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	timeoutChan := make(chan int)

	go func() {
		wg.Wait()
		timeoutChan <- 3
		close(timeoutChan)
	}()

	select {
	case <-timeoutChan:
		return false
	case <-time.After(timeout):
		return true
	}
}

func TestInitOk(t *testing.T) {
	tcpConn := NewFakeTCPConn(false, nil)
	ttyConn := common.NewTTYSenderConnection(tcpConn)
	defer ttyConn.Close()

	senderSessionInfo := common.SenderSessionInfo{
		Salt:              fmt.Sprintf("salt_%d", time.Now().UnixNano()),
		PasswordVerifierA: fmt.Sprintf("pass_a_%d", time.Now().UnixNano()),
	}
	serverSessionInfo := common.ServerSessionInfo{
		URLWebReadWrite: fmt.Sprintf("http://%x:", time.Now().UnixNano()),
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		err, res := ttyConn.InitSender(senderSessionInfo)

		if err != nil {
			panic(fmt.Sprintf("Can't initialise the sender side: %s", err.Error()))
		}

		if res.URLWebReadWrite != serverSessionInfo.URLWebReadWrite {
			panic(fmt.Sprintf("Received URL different from expected: <%s> != <%s>",
				res.URLWebReadWrite, serverSessionInfo.URLWebReadWrite))
		}
	}()

	err, res := ttyConn.InitServer(serverSessionInfo)

	if err != nil {
		t.Fatalf("Can't Initialise the server side: %s", err.Error())
	}

	if res.PasswordVerifierA != senderSessionInfo.PasswordVerifierA || res.Salt != senderSessionInfo.Salt {
		t.Fatalf("Received invalid sender session info: <%s> != <%s>, <%s> != <%s> ",
			res.PasswordVerifierA, senderSessionInfo.PasswordVerifierA, res.Salt, senderSessionInfo.Salt)
	}

	if wgWaitTimeout(&wg, 10*time.Millisecond) {
		t.Fatalf("Waiting for initialisation took too long")
	}
}

func TestInitServerBrokenConnection(t *testing.T) {
	tcpConn := NewFakeTCPConn(false, nil)
	ttyConn := common.NewTTYSenderConnection(tcpConn)
	defer ttyConn.Close()

	serverSessionInfo := common.ServerSessionInfo{
		URLWebReadWrite: fmt.Sprintf("http://%x:", time.Now().UnixNano()),
	}
	senderSessionInfo := common.SenderSessionInfo{
		Salt:              fmt.Sprintf("salt_%d", time.Now().UnixNano()),
		PasswordVerifierA: fmt.Sprintf("pass_a_%d", time.Now().UnixNano()),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// This should make the InitServer and InitSender fail
	tcpConn.Close()

	go func() {
		defer wg.Done()
		err, _ := ttyConn.InitServer(serverSessionInfo)

		if err == nil {
			panic("Expected the connection to fail, but it didn't")
		}
	}()

	go func() {
		defer wg.Done()
		err, _ := ttyConn.InitSender(senderSessionInfo)

		if err == nil {
			panic("Expected the connection to fail, but it didn't")
		}
	}()

	// Timeout the test
	if wgWaitTimeout(&wg, 500*time.Millisecond) {
		t.Fatalf("Waiting for initialisation took too long")
	}
}

func TestInitBrokenWrite(t *testing.T) {
	brokenWrite := func(writer io.Writer, b []byte) (int, error) {
		// Write just half of the bytes
		return writer.Write(b[:len(b)/2])
	}
	tcpConn := NewFakeTCPConn(false, brokenWrite)
	ttyConn := common.NewTTYSenderConnection(tcpConn)
	ttyConn.SetDeadline(time.Now().Add(time.Second * 1))
	defer ttyConn.Close()

	senderSessionInfo := common.SenderSessionInfo{
		Salt:              fmt.Sprintf("salt_%d", time.Now().UnixNano()),
		PasswordVerifierA: fmt.Sprintf("pass_a_%d", time.Now().UnixNano()),
	}
	serverSessionInfo := common.ServerSessionInfo{
		URLWebReadWrite: fmt.Sprintf("http://%x:", time.Now().UnixNano()),
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		err, _ := ttyConn.InitSender(senderSessionInfo)

		if err == nil {
			panic("Expected error when InitSender, but got nil")
		}
	}()

	go func() {
		defer wg.Done()
		err, _ := ttyConn.InitServer(serverSessionInfo)

		if err == nil {
			panic("Expected error when InitServer, but got nil")
		}
	}()

	if wgWaitTimeout(&wg, 2*time.Second) {
		t.Fatalf("Waiting for initialisation took too long")
	}
}

func TestWriteOk(t *testing.T) {
	client, server := NewDoubleNetConn(false)
	ttyConnC := common.NewTTYSenderConnection(client)
	ttyConnS := common.NewTTYSenderConnection(server)
	defer ttyConnC.Close()
	defer ttyConnS.Close()

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()

		go func() {
			defer wg.Done()

			// If HandleReceive() returns an error, it must be because of closing the connection.
			// If not, it will be caught anyways when comparing the actual data.
			for {
				err := ttyConnC.HandleReceive()
				if err != nil {
					break
				}
			}
		}()

		buff := make([]byte, 1024)
		for i := 0; i < 100; i++ {
			data := fmt.Sprintf("Data %d", i)
			nw, errw := ttyConnC.Write([]byte(data))
			nr, errr := ttyConnC.Read(buff)

			if errw != nil {
				panic(fmt.Sprintf("Couldn't write: %s", errw.Error()))
			}
			if errr != nil {
				panic(fmt.Sprintf("Couldn't read: %s", errr.Error()))
			}
			if nr != nw {
				panic(fmt.Sprintf("Unexpected number if bytes written and read: %d != %d", nw, nr))
			}

			rcvData := string(buff[:nr])
			if data != rcvData {
				panic(fmt.Sprintf("Unexpected data: expected vs expected: <%s> != <%s>", data, rcvData))
			}
		}

		ttyConnC.Close()
	}()

	go func() {
		defer wg.Done()

		go func() {
			defer wg.Done()
			// If HandleReceive() returns an error, it must be because of closing the connection.
			// If not, it will be caught anyways when comparing the actual data.
			for {
				err := ttyConnS.HandleReceive()
				if err != nil {
					break
				}
			}
		}()

		// Make the server echo back what it received
		io.Copy(ttyConnS, ttyConnS)
	}()

	if wgWaitTimeout(&wg, 3*time.Second) {
		t.Fatalf("Timed out")
	}

}
