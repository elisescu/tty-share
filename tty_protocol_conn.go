package main

import (
	"encoding/json"
	"io"
)

type ServerSessionInfo struct {
	URLWebReadWrite string
}

type ReceiverSessionInfo struct {
}

type SenderSessionInfo struct {
	Salt              string
	PasswordVerifierA string
}

// TTYProtocolConn is the interface used to communicate with the sending (master) side of the TTY session
type TTYProtocolConn struct {
	netConnection io.ReadWriteCloser
	jsonDecoder   *json.Decoder
}

func NewTTYProtocolConn(conn io.ReadWriteCloser) *TTYProtocolConn {
	return &TTYProtocolConn{
		netConnection: conn,
		jsonDecoder:   json.NewDecoder(conn),
	}
}

func (protoConn *TTYProtocolConn) ReadMessage() (msg MsgAll, err error) {
	// TODO: perhaps read here the error, and transform it to something that's understandable
	// from the outside in the context of this object
	err = protoConn.jsonDecoder.Decode(&msg)
	return
}

func (protoConn *TTYProtocolConn) SetWinSize(cols, rows int) error {
	msgWinChanged := MsgTTYWinSize{
		Cols: cols,
		Rows: rows,
	}
	return MarshalAndWriteMsg(protoConn.netConnection, msgWinChanged)
}

func (protoConn *TTYProtocolConn) Close() error {
	return protoConn.netConnection.Close()
}

// Function to send data from one the sender to the server and the other way around.
func (protoConn *TTYProtocolConn) Write(buff []byte) (int, error) {
	msgWrite := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}
	return len(buff), MarshalAndWriteMsg(protoConn.netConnection, msgWrite)
}

func (protoConn *TTYProtocolConn) WriteRawData(buff []byte) (int, error) {
	return protoConn.netConnection.Write(buff)
}

// Function to be called on the sender side, and which blocks until the protocol has been
// initialised
func (protoConn *TTYProtocolConn) InitSender(senderInfo SenderSessionInfo) (serverInfo ServerSessionInfo, err error) {
	var replyMsg MsgTTYSenderInitReply

	msgInitReq := MsgTTYSenderInitRequest{
		Salt:              senderInfo.Salt,
		PasswordVerifierA: senderInfo.PasswordVerifierA,
	}

	// Send the InitRequest message
	if err = MarshalAndWriteMsg(protoConn.netConnection, msgInitReq); err != nil {
		return
	}

	// Wait here for the InitReply message
	if err = ReadAndUnmarshalMsg(protoConn.netConnection, &replyMsg); err != nil {
		return
	}

	serverInfo = ServerSessionInfo{
		URLWebReadWrite: replyMsg.ReceiverURLWebReadWrite,
	}
	return
}

func (protoConn *TTYProtocolConn) InitServer(serverInfo ServerSessionInfo) (senderInfo SenderSessionInfo, err error) {
	var requestMsg MsgTTYSenderInitRequest

	// Wait here and expect a InitRequest message
	if err = ReadAndUnmarshalMsg(protoConn.netConnection, &requestMsg); err != nil {
		return
	}

	// Send back a InitReply message
	if err = MarshalAndWriteMsg(protoConn.netConnection, MsgTTYSenderInitReply{
		ReceiverURLWebReadWrite: serverInfo.URLWebReadWrite}); err != nil {
		return
	}

	senderInfo = SenderSessionInfo{
		Salt:              requestMsg.Salt,
		PasswordVerifierA: requestMsg.PasswordVerifierA,
	}
	return
}

func (protoConn *TTYProtocolConn) InitServerReceiverConn(serverInfo ServerSessionInfo) (receiverInfo ReceiverSessionInfo, err error) {
	return
}

func (protoConn *TTYProtocolConn) InitReceiverServerConn(receiverInfo ReceiverSessionInfo) (serverInfo ServerSessionInfo, err error) {
	return
}
