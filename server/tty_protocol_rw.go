package server

import (
	"encoding/json"
	"io"

	"github.com/gorilla/websocket"
)

const (
	MsgIDWrite   = "Write"
	MsgIDWinSize = "WinSize"
)

// Message used to encapsulate the rest of the bessages bellow
type MsgWrapper struct {
	Type string
	Data []byte
}

type MsgTTYWrite struct {
	Data []byte
	Size int
}

type MsgTTYWinSize struct {
	Cols int
	Rows int
}

type OnMsgWrite func(data []byte)
type OnMsgWinSize func(cols, rows int)

type TTYProtocolWS struct {
	ws      *websocket.Conn
}

func NewTTYProtocolWS(ws *websocket.Conn) *TTYProtocolWS {
	return &TTYProtocolWS{
		ws:      ws,
	}
}

func marshalMsg(aMessage interface{}) (_ []byte, err error) {
	var msg MsgWrapper

	if writeMsg, ok := aMessage.(MsgTTYWrite); ok {
		msg.Type = MsgIDWrite
		msg.Data, err = json.Marshal(writeMsg)
		//fmt.Printf("Sent write message %s\n", string(writeMsg.Data))
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

	if winChangedMsg, ok := aMessage.(MsgTTYWinSize); ok {
		msg.Type = MsgIDWinSize
		msg.Data, err = json.Marshal(winChangedMsg)
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

	return nil, nil
}


func (handler *TTYProtocolWS) ReadAndHandle(onWrite OnMsgWrite, onWinSize OnMsgWinSize) (err error) {
	var msg MsgWrapper

	_, r, err := handler.ws.NextReader()
	if err != nil {
		// underlaying conn is closed. signal that through io.EOF
		return io.EOF
	}

	err = json.NewDecoder(r).Decode(&msg)

	if err != nil {
		return
	}

	switch msg.Type {
	case MsgIDWrite:
		var msgWrite MsgTTYWrite
		err = json.Unmarshal(msg.Data, &msgWrite)
		if err == nil {
			onWrite(msgWrite.Data)
		}
	case MsgIDWinSize:
		var msgRemoteWinSize MsgTTYWinSize
		err = json.Unmarshal(msg.Data, &msgRemoteWinSize)
		if err == nil {
			onWinSize(msgRemoteWinSize.Cols, msgRemoteWinSize.Rows)
		}
	}
	return
}

func (handler *TTYProtocolWS) SetWinSize(cols, rows int) error {
	msgWinChanged := MsgTTYWinSize{
		Cols: cols,
		Rows: rows,
	}
	data, err := marshalMsg(msgWinChanged)
	if err != nil {
		return err
	}

	return handler.ws.WriteMessage(websocket.TextMessage, data)
}

// Function to send data from one the sender to the server and the other way around.
func (handler *TTYProtocolWS) Write(buff []byte) (int, error) {
	msgWrite := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}
	data, err := marshalMsg(msgWrite)
	if err != nil {
		return 0, err
	}

	return len(buff), handler.ws.WriteMessage(websocket.TextMessage, data)
}
