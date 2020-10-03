package server

import (
	"container/list"
	"encoding/json"

	"io"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ttyShareSession struct {
	mainRWLock             sync.RWMutex
	ttyReceiverConnections *list.List
	isAlive                bool
	lastWindowSizeMsg      MsgTTYWinSize
	ttyWriter              io.Writer
}

func newTTYShareSession(ttyWriter io.Writer) *ttyShareSession {

	ttyShareSession := &ttyShareSession{
		ttyReceiverConnections: list.New(),
		ttyWriter:              ttyWriter,
	}

	return ttyShareSession
}

func copyList(l *list.List) *list.List {
	newList := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		newList.PushBack(e.Value)
	}
	return newList
}

func (session *ttyShareSession) WindowSize(cols, rows int) (err error) {
	msg := MsgTTYWinSize{
		Cols: cols,
		Rows: rows,
	}

	session.mainRWLock.Lock()
	session.lastWindowSizeMsg = msg
	session.mainRWLock.Unlock()

	data, _ := MarshalMsg(msg)

	session.forEachReceiverLock(func(rcvConn *TTYProtocolWriter) bool {
		_, e := rcvConn.WriteRawData(data)
		if e != nil {
			err = e
		}
		return true
	})
	return
}

func (session *ttyShareSession) Write(buff []byte) (written int, err error) {
	msg := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}

	data, _ := MarshalMsg(msg)

	session.forEachReceiverLock(func(rcvConn *TTYProtocolWriter) bool {
		_, e := rcvConn.WriteRawData(data)
		if e != nil {
			err = e
		}
		return true
	})

	// TODO: fix this
	written = len(buff)
	return
}

// Runs the callback cb for each of the receivers in the list of the receivers, as it was when
// this function was called. Note that there might be receivers which might have lost
// the connection since this function was called.
// Return false in the callback to not continue for the rest of the receivers
func (session *ttyShareSession) forEachReceiverLock(cb func(rcvConn *TTYProtocolWriter) bool) {
	session.mainRWLock.RLock()
	// TODO: Maybe find a better way?
	rcvsCopy := copyList(session.ttyReceiverConnections)
	session.mainRWLock.RUnlock()

	for receiverE := rcvsCopy.Front(); receiverE != nil; receiverE = receiverE.Next() {
		receiver := receiverE.Value.(*TTYProtocolWriter)
		if !cb(receiver) {
			break
		}
	}
}

// quick and dirty locked writer
type lockedWriter struct {
	writer io.Writer
	lock sync.Mutex
}

func (wl *lockedWriter) Write(data []byte) (int, error) {
	wl.lock.Lock()
	defer wl.lock.Unlock()
	return wl.writer.Write(data)
}

// Will run on the TTYReceiver connection go routine (e.g.: on the websockets connection routine)
// When HandleWSConnection will exit, the connection to the TTYReceiver will be closed
func (session *ttyShareSession) HandleWSConnection(wsConn *WSConnection) {
	rcvReader := NewTTYProtocolReader(wsConn)

	// Gorilla websockets don't allow for concurent writes. Lazy, and perhaps shorter solution
	// is to wrap a lock around a writer. Maybe later replace it with a channel
	rcvWriter := NewTTYProtocolWriter(&lockedWriter{
		writer: wsConn,
	})

	// Add the receiver to the list of receivers in the seesion, so we need to write-lock
	session.mainRWLock.Lock()
	rcvHandleEl := session.ttyReceiverConnections.PushBack(rcvWriter)
	lastWindowSizeData, _ := MarshalMsg(session.lastWindowSizeMsg)
	session.mainRWLock.Unlock()

	log.Debugf("New WS connection (%s). Serving ..", wsConn.Address())

	// Sending the initial size of the window, if we have one
	rcvWriter.WriteRawData(lastWindowSizeData)

	// Wait until the TTYReceiver will close the connection on its end
	for {
		msg, err := rcvReader.ReadMessage()
		if err != nil {
			log.Debugf("Finished the WS reading loop: %s", err.Error())
			break
		}

		// We only support MsgTTYWrite from the web terminal for now
		if msg.Type != MsgIDWrite {
			log.Warnf("Unknown message over the WS connection: type %s", msg.Type)
			break
		}

		var msgW MsgTTYWrite
		json.Unmarshal(msg.Data, &msgW)
		session.ttyWriter.Write(msgW.Data)
	}

	// Remove the recevier from the list of the receiver of this session, so we need to write-lock
	session.mainRWLock.Lock()
	session.ttyReceiverConnections.Remove(rcvHandleEl)
	session.mainRWLock.Unlock()

	wsConn.Close()
	log.Debugf("Closed receiver connection")
}
