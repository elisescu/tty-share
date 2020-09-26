package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type TTYProtocolReader struct {
	reader  io.Reader
	jsonDecoder *json.Decoder
}

type TTYProtocolWriter struct {
	writer  io.Writer
}

const (
	MsgIDWrite   = "Write"
	MsgIDWinSize = "WinSize"
)

// Message used to encapsulate the rest of the bessages bellow
type MsgAll struct {
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

func ReadAndUnmarshalMsg(reader io.Reader, aMessage interface{}) (err error) {
	var wrapperMsg MsgAll
	// Wait here for the right message to come
	dec := json.NewDecoder(reader)
	err = dec.Decode(&wrapperMsg)

	if err != nil {
		return errors.New("Cannot decode top message: " + err.Error())
	}

	err = json.Unmarshal(wrapperMsg.Data, aMessage)

	if err != nil {
		return errors.New("Cannot decode message: " + err.Error() + string(wrapperMsg.Data))
	}
	return
}

func MarshalMsg(aMessage interface{}) (_ []byte, err error) {
	var msg MsgAll

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

func NewTTYProtocolWriter(w io.Writer) *TTYProtocolWriter {
	return &TTYProtocolWriter{
		writer:  w,
	}
}

func NewTTYProtocolReader(r io.Reader) *TTYProtocolReader {
	return &TTYProtocolReader{
		reader:  r,
		jsonDecoder: json.NewDecoder(r),
	}
}

func (reader *TTYProtocolReader) ReadMessage() (msg MsgAll, err error) {
	// TODO: perhaps read here the error, and transform it to something that's understandable
	// from the outside in the context of this object
	err = reader.jsonDecoder.Decode(&msg)
	return
}

func (writer *TTYProtocolWriter) SetWinSize(cols, rows int) error {
	msgWinChanged := MsgTTYWinSize{
		Cols: cols,
		Rows: rows,
	}
	return MarshalAndWriteMsg(writer.writer, msgWinChanged)
}

// Function to send data from one the sender to the server and the other way around.
func (writer *TTYProtocolWriter) Write(buff []byte) (int, error) {
	msgWrite := MsgTTYWrite{
		Data: buff,
		Size: len(buff),
	}
	return len(buff), MarshalAndWriteMsg(writer.writer, msgWrite)
}

func (writer *TTYProtocolWriter) WriteRawData(buff []byte) (int, error) {
	return writer.writer.Write(buff)
}
