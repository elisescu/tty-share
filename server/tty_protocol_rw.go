package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/elisescu/tty-share/crypto"
	"github.com/gorilla/websocket"
)

const (
	MsgIDWrite     = "Write"
	MsgIDWinSize   = "WinSize"
	MsgIDEncrypted = "Encrypted"
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

type MsgEncrypted struct {
	EncryptedData []byte
	Nonce         []byte
}

type OnMsgWrite func(data []byte)
type OnMsgWinSize func(cols, rows int)
type OnMsgEncrypted func() // Called when an encrypted message is detected

type TTYProtocolWSLocked struct {
	ws            *websocket.Conn
	lock          sync.Mutex
	encryptionKey []byte // nil if encryption is disabled
}

func NewTTYProtocolWSLocked(ws *websocket.Conn, encryptionKey []byte) *TTYProtocolWSLocked {
	return &TTYProtocolWSLocked{
		ws:            ws,
		encryptionKey: encryptionKey,
	}
}

func (handler *TTYProtocolWSLocked) MarshalMsg(aMessage interface{}) (_ []byte, err error) {
	var msg MsgWrapper

	// If encryption is enabled, wrap the message in an encrypted envelope
	if handler.encryptionKey != nil {
		var plainData []byte
		
		if writeMsg, ok := aMessage.(MsgTTYWrite); ok {
			plainData, err = json.Marshal(writeMsg)
			if err != nil {
				return nil, err
			}
		} else if winChangedMsg, ok := aMessage.(MsgTTYWinSize); ok {
			plainData, err = json.Marshal(winChangedMsg)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, nil
		}

		// Encrypt the message data
		encryptedData, nonce, err := crypto.EncryptData(plainData, handler.encryptionKey)
		if err != nil {
			return nil, err
		}

		// Create encrypted message envelope
		msg.Type = MsgIDEncrypted
		encryptedMsg := MsgEncrypted{
			EncryptedData: encryptedData,
			Nonce:         nonce,
		}
		msg.Data, err = json.Marshal(encryptedMsg)
		if err != nil {
			return nil, err
		}
		
		return json.Marshal(msg)
	}

	// Unencrypted mode (original behavior)
	if writeMsg, ok := aMessage.(MsgTTYWrite); ok {
		msg.Type = MsgIDWrite
		msg.Data, err = json.Marshal(writeMsg)
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

func (handler *TTYProtocolWSLocked) ReadAndHandle(onWrite OnMsgWrite, onWinSize OnMsgWinSize, onEncrypted OnMsgEncrypted) (err error) {
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
	case MsgIDEncrypted:
		// Call the encrypted message callback if provided
		if onEncrypted != nil {
			onEncrypted()
		}
		
		// Handle encrypted message
		if handler.encryptionKey != nil {
			var encryptedMsg MsgEncrypted
			err = json.Unmarshal(msg.Data, &encryptedMsg)
			if err != nil {
				return err
			}

			// Decrypt the message
			plainData, err := crypto.DecryptData(encryptedMsg.EncryptedData, encryptedMsg.Nonce, handler.encryptionKey)
			if err != nil {
				return err
			}

			// The decrypted data is the original message JSON, so we need to determine its type
			// by attempting to unmarshal into both possible message types
			
			// Try to unmarshal as MsgTTYWrite first
			var msgWrite MsgTTYWrite
			err = json.Unmarshal(plainData, &msgWrite)
			if err == nil && msgWrite.Size > 0 {
				onWrite(msgWrite.Data)
			} else {
				// Try to unmarshal as MsgTTYWinSize
				var msgRemoteWinSize MsgTTYWinSize
				err = json.Unmarshal(plainData, &msgRemoteWinSize)
				if err == nil && (msgRemoteWinSize.Cols > 0 || msgRemoteWinSize.Rows > 0) {
					onWinSize(msgRemoteWinSize.Cols, msgRemoteWinSize.Rows)
				}
			}
		} else {
			// No encryption key - show encrypted data as-is
			var encryptedMsg MsgEncrypted
			err = json.Unmarshal(msg.Data, &encryptedMsg)
			if err != nil {
				return err
			}
			
			// Display encrypted data as base64 text for user to see
			encryptedText := fmt.Sprintf("[ENCRYPTED] %s", base64.StdEncoding.EncodeToString(encryptedMsg.EncryptedData))
			onWrite([]byte(encryptedText))
		}
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

func (handler *TTYProtocolWSLocked) SetWinSize(cols, rows int) (err error) {
	msgWinChanged := MsgTTYWinSize{
		Cols: cols,
		Rows: rows,
	}
	data, err := handler.MarshalMsg(msgWinChanged)
	if err != nil {
		return
	}

	handler.lock.Lock()
	err = handler.ws.WriteMessage(websocket.TextMessage, data)
	handler.lock.Unlock()
	return
}

// Function to send data from one the sender to the server and the other way around.
func (handler *TTYProtocolWSLocked) Write(buff []byte) (n int, err error) {
	msgWrite := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}
	data, err := handler.MarshalMsg(msgWrite)
	if err != nil {
		return 0, err
	}

	handler.lock.Lock()
	n, err = len(buff), handler.ws.WriteMessage(websocket.TextMessage, data)
	handler.lock.Unlock()
	return
}
