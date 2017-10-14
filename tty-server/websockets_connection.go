package main

import (
	"github.com/gorilla/websocket"
)

type WSConnection struct {
	connection *websocket.Conn
	address    string
}

func newWSConnection(conn *websocket.Conn) *WSConnection {
	return &WSConnection{
		connection: conn,
		address:    conn.RemoteAddr().String(),
	}
}

func (handle *WSConnection) Write(data []byte) (n int, err error) {
	w, err := handle.connection.NextWriter(websocket.TextMessage)
	if err != nil {
		return 0, err
	}
	n, err = w.Write(data)
	w.Close()
	return
}

func (handle *WSConnection) Close() (err error) {
	return handle.connection.Close()
}

func (handle *WSConnection) Address() string {
	return handle.address
}

func (handle *WSConnection) Read(data []byte) (int, error) {
	_, r, err := handle.connection.NextReader()
	if err != nil {
		return 0, err
	}
	return r.Read(data)
}
