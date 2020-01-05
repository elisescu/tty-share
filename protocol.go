package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type ProtocolMessageIDType string

const (
	MsgIDSenderInitRequest          = "SenderInitRequest"
	MsgIDSenderInitReply            = "SenderInitReply"
	MsgIDSenderNewReceiverConnected = "SenderNewReceiverConnected"
	MsgIDReceiverInitRequest        = "ReceiverInitRequest"
	MsgIDReceiverInitReply          = "ReceiverInitReply"
	MsgIDWrite                      = "Write"
	MsgIDWinSize                    = "WinSize"
)

// Message used to encapsulate the rest of the bessages bellow
type MsgAll struct {
	Type ProtocolMessageIDType
	Data []byte
}

// These messages are used between the server and the sender/receiver
type MsgTTYSenderInitRequest struct {
	Salt              string
	PasswordVerifierA string
}

type MsgTTYSenderInitReply struct {
	ReceiverURLWebReadWrite string
}

type MsgTTYSenderNewReceiverConnected struct {
	Name string
}

type MsgTTYReceiverInitRequest struct {
	ChallengeReply string
}

type MsgTTYReceiverInitReply struct {
}

// These messages are not intended for the server, so they are just forwarded by it to the remote
// side.
type MsgTTYWrite struct {
	Data []byte
	Size int
}

type MsgTTYWinSize struct {
	Cols int
	Rows int
}

func ReadAndUnmarshalMsg(reader io.Reader, aMessage interface{}) (err error) {
	var wrapperMsg MsgAll
	// Wait here for the right message to come
	dec := json.NewDecoder(reader)
	err = dec.Decode(&wrapperMsg)

	if err != nil {
		return errors.New("Cannot decode message: " + err.Error())
	}

	err = json.Unmarshal(wrapperMsg.Data, aMessage)

	if err != nil {
		return errors.New("Cannot decode message: " + err.Error())
	}
	return
}

func MarshalMsg(aMessage interface{}) (_ []byte, err error) {
	var msg MsgAll

	if initRequestMsg, ok := aMessage.(MsgTTYSenderInitRequest); ok {
		msg.Type = MsgIDSenderInitRequest
		msg.Data, err = json.Marshal(initRequestMsg)
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

	if initReplyMsg, ok := aMessage.(MsgTTYSenderInitReply); ok {
		msg.Type = MsgIDSenderInitReply
		msg.Data, err = json.Marshal(initReplyMsg)
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

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

	if newRcvMsg, ok := aMessage.(MsgTTYSenderNewReceiverConnected); ok {
		msg.Type = MsgIDSenderNewReceiverConnected
		msg.Data, err = json.Marshal(newRcvMsg)
		if err != nil {
			return
		}
		return json.Marshal(msg)
	}

	return nil, nil
}

func MarshalAndWriteMsg(writer io.Writer, aMessage interface{}) (err error) {
	b, err := MarshalMsg(aMessage)

	if err != nil {
		return
	}

	n, err := writer.Write(b)

	if n != len(b) {
		err = fmt.Errorf("Unable to write : wrote %d out of %d bytes", n, len(b))
		return
	}

	if err != nil {
		return
	}

	return
}
