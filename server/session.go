package server

import (
	"container/list"
	"io"
	"sync"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type ttyShareSession struct {
	mainRWLock          sync.RWMutex
	ttyProtoConnections *list.List
	isAlive             bool
	lastWindowSizeMsg   MsgTTYWinSize
	ptyHandler           PTYHandler
}

// quick and dirty locked writer
type lockedWriter struct {
	writer io.Writer
	lock   sync.Mutex
}

func (wl *lockedWriter) Write(data []byte) (int, error) {
	wl.lock.Lock()
	defer wl.lock.Unlock()
	return wl.writer.Write(data)
}

func copyList(l *list.List) *list.List {
	newList := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		newList.PushBack(e.Value)
	}
	return newList
}

func newTTYShareSession(ptyHandler PTYHandler) *ttyShareSession {

	ttyShareSession := &ttyShareSession{
		ttyProtoConnections: list.New(),
		ptyHandler:           ptyHandler,
	}

	return ttyShareSession
}

func (session *ttyShareSession) WindowSize(cols, rows int) error {
	session.mainRWLock.Lock()
	session.lastWindowSizeMsg = MsgTTYWinSize{Cols: cols, Rows: rows}
	session.mainRWLock.Unlock()

	session.forEachReceiverLock(func(rcvConn *TTYProtocolWS) bool {
		rcvConn.SetWinSize(cols, rows)
		return true
	})
	return nil
}

func (session *ttyShareSession) Write(data []byte) (int, error) {
	session.forEachReceiverLock(func(rcvConn *TTYProtocolWS) bool {
		rcvConn.Write(data)
		return true
	})
	return len(data), nil
}

// Runs the callback cb for each of the receivers in the list of the receivers, as it was when
// this function was called. Note that there might be receivers which might have lost
// the connection since this function was called.
// Return false in the callback to not continue for the rest of the receivers
func (session *ttyShareSession) forEachReceiverLock(cb func(rcvConn *TTYProtocolWS) bool) {
	session.mainRWLock.RLock()
	// TODO: Maybe find a better way?
	rcvsCopy := copyList(session.ttyProtoConnections)
	session.mainRWLock.RUnlock()

	for receiverE := rcvsCopy.Front(); receiverE != nil; receiverE = receiverE.Next() {
		receiver := receiverE.Value.(*TTYProtocolWS)
		if !cb(receiver) {
			break
		}
	}
}

// Will run on the TTYReceiver connection go routine (e.g.: on the websockets connection routine)
// When HandleWSConnection will exit, the connection to the TTYReceiver will be closed
func (session *ttyShareSession) HandleWSConnection(wsConn *websocket.Conn) {
	protoConn := NewTTYProtocolWS(wsConn)

	session.mainRWLock.Lock()
	rcvHandleEl := session.ttyProtoConnections.PushBack(protoConn)
	winSize := session.lastWindowSizeMsg
	session.mainRWLock.Unlock()

	log.Debugf("New WS connection (%s). Serving ..", wsConn.RemoteAddr().String())

	// Sending the initial size of the window, if we have one
	protoConn.SetWinSize(winSize.Cols, winSize.Rows)

	// Wait until the TTYReceiver will close the connection on its end
	for {
		err := protoConn.ReadAndHandle(
			func(data []byte) {
				session.ptyHandler.Write(data)
			},
			func(cols, rows int) {
				session.ptyHandler.Refresh()
			},
		)

		if err != nil {
			log.Debugf("Finished the WS reading loop: %s", err.Error())
			break
		}
	}

	// Remove the recevier from the list of the receiver of this session, so we need to write-lock
	session.mainRWLock.Lock()
	session.ttyProtoConnections.Remove(rcvHandleEl)
	session.mainRWLock.Unlock()

	wsConn.Close()
	log.Debugf("Closed receiver connection")
}
