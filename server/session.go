package main

import (
	"container/list"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync"
)

type sessionInfo struct {
	ID              string
	URLWebReadWrite string
}

type ttyShareSession struct {
	sessionID              string
	serverURL              string
	mainRWLock             sync.RWMutex
	ttySenderConnection    *TTYProtocolConn
	ttyReceiverConnections *list.List
	isAlive                bool
	lastWindowSizeMsg      MsgAll
}

func generateNewSessionID() string {
	binID := make([]byte, 32)
	_, err := rand.Read(binID)

	if err != nil {
		panic(err)
	}

	return base64.URLEncoding.EncodeToString([]byte(binID))
}

func newTTYShareSession(conn net.Conn, serverURL string) *ttyShareSession {
	sessionID := generateNewSessionID()

	ttyShareSession := &ttyShareSession{
		sessionID:              sessionID,
		serverURL:              serverURL,
		ttySenderConnection:    NewTTYProtocolConn(conn),
		ttyReceiverConnections: list.New(),
	}

	return ttyShareSession
}

func (session *ttyShareSession) InitSender() error {
	_, err := session.ttySenderConnection.InitServer(ServerSessionInfo{
		URLWebReadWrite: session.serverURL + "/s/" + session.GetID(),
	})
	return err
}

func (session *ttyShareSession) GetID() string {
	return session.sessionID
}

func copyList(l *list.List) *list.List {
	newList := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		newList.PushBack(e.Value)
	}
	return newList
}

func (session *ttyShareSession) handleSenderMessageLock(msg MsgAll) {
	switch msg.Type {
	case MsgIDWinSize:
		// Save the last known size of the window so we pass it to new receivers, and then
		// fallthrough. We save the WinSize message as we get it, since we send it anyways
		// to the receivers, packed into the same protocol
		session.mainRWLock.Lock()
		session.lastWindowSizeMsg = msg
		session.mainRWLock.Unlock()
		fallthrough
	case MsgIDWrite:
		data, _ := json.Marshal(msg)
		session.forEachReceiverLock(func(rcvConn *TTYProtocolConn) bool {
			rcvConn.WriteRawData(data)
			return true
		})
	}
}

// Will run on the ttySendeConnection go routine (e.g.: in the TCP connection routine)
func (session *ttyShareSession) HandleSenderConnection() {
	session.mainRWLock.Lock()
	session.isAlive = true
	senderConnection := session.ttySenderConnection
	session.mainRWLock.Unlock()

	for {
		msg, err := senderConnection.ReadMessage()
		if err != nil {
			log.Debugf("TTYSender connnection finished withs with error: %s", err.Error())
			break
		}
		session.handleSenderMessageLock(msg)
	}

	// Close the connection to all the receivers
	log.Debugf("Closing all receiver connection")
	session.forEachReceiverLock(func(recvConn *TTYProtocolConn) bool {
		log.Debugf("Closing receiver connection")
		recvConn.Close()
		return true
	})

	// TODO: clear here the list of receiver
	session.mainRWLock.Lock()
	session.isAlive = false
	session.mainRWLock.Unlock()
}

// Runs the callback cb for each of the receivers in the list of the receivers, as it was when
// this function was called. Note that there might be receivers which might have lost
// the connection since this function was called.
// Return false in the callback to not continue for the rest of the receivers
func (session *ttyShareSession) forEachReceiverLock(cb func(rcvConn *TTYProtocolConn) bool) {
	session.mainRWLock.RLock()
	// TODO: Maybe find a better way?
	rcvsCopy := copyList(session.ttyReceiverConnections)
	session.mainRWLock.RUnlock()

	for receiverE := rcvsCopy.Front(); receiverE != nil; receiverE = receiverE.Next() {
		receiver := receiverE.Value.(*TTYProtocolConn)
		if !cb(receiver) {
			break
		}
	}
}

// Will run on the TTYReceiver connection go routine (e.g.: on the websockets connection routine)
// When HandleReceiver will exit, the connection to the TTYReceiver will be closed
func (session *ttyShareSession) HandleReceiver(rawConn *WSConnection) {
	rcvProtoConn := NewTTYProtocolConn(rawConn)

	session.mainRWLock.Lock()
	if !session.isAlive {
		log.Warnf("TTYReceiver tried to connect to a session that is not alive anymore. Rejecting it..")
		session.mainRWLock.Unlock()
		return
	}

	// Add the receiver to the list of receivers in the seesion, so we need to write-lock
	rcvHandleEl := session.ttyReceiverConnections.PushBack(rcvProtoConn)
	senderConn := session.ttySenderConnection
	lastWindowSize, _ := json.Marshal(session.lastWindowSizeMsg)
	session.mainRWLock.Unlock()

	log.Debugf("Got new TTYReceiver connection (%s). Serving it..", rawConn.Address())

	// Sending the initial size of the window, if we have one
	rcvProtoConn.WriteRawData(lastWindowSize)

	// Notify the tty-share that we got a new receiver connected
	msgRcvConnected, err := MarshalMsg(MsgTTYSenderNewReceiverConnected{
		Name: rawConn.Address(),
	})
	senderConn.WriteRawData(msgRcvConnected)

	if err != nil {
		log.Errorf("Cannot notify tty sender. Error: %s", err.Error())
	}

	// Wait until the TTYReceiver will close the connection on its end
	for {
		msg, err := rcvProtoConn.ReadMessage()

		if err != nil {
			log.Warnf("Finishing handling the TTYReceiver loop because: %s", err.Error())
			break
		}

		switch msg.Type {
		case MsgIDWinSize:
			// Ignore these messages from the receiver. For now, the policy is that the sender
			// decides on the window size.
		case MsgIDWrite:
			rawData, _ := json.Marshal(msg)
			senderConn.WriteRawData(rawData)
		default:
			log.Warnf("Receiving unknown data from the receiver")
		}
	}

	log.Debugf("Closing receiver connection")
	rcvProtoConn.Close()

	// Remove the recevier from the list of the receiver of this session, so we need to write-lock
	session.mainRWLock.Lock()
	session.ttyReceiverConnections.Remove(rcvHandleEl)
	session.mainRWLock.Unlock()
}
