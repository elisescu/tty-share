package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	logrus "github.com/sirupsen/logrus"
)

var log = logrus.New()
var version string = "0.0.0"
var config = TTYShareConfig{ // default configuration options
	LogFileName: "-",
	Server:      "go.tty-share.com:7654",
	UseTLS:      false,
}

func main() {
	config.Load()
	log.Level = logrus.ErrorLevel
	if config.LogFileName != "-" {
		fmt.Printf("Writing logs to: %s\n", config.LogFileName)
		logFile, err := os.Create(config.LogFileName)
		if err != nil {
			fmt.Printf("Can't open %s for writing logs\n", config.LogFileName)
		}
		log.Level = logrus.DebugLevel
		log.Out = logFile
	}

	if !isStdinTerminal() {
		fmt.Printf("Input not a tty\n")
		os.Exit(1)
	}

	var rawConnection io.ReadWriteCloser
	if config.UseTLS {
		roots, err := x509.SystemCertPool()
		if err != nil {
			fmt.Printf("Cannot connect to the server (%s): %s", config.Server, err.Error())
			return
		}
		rawConnection, err = tls.Dial("tcp", config.Server, &tls.Config{RootCAs: roots})
		if err != nil {
			fmt.Printf("Cannot connect (TLS) to the server (%s): %s", config.Server, err.Error())
			return
		}
	} else {
		var err error
		rawConnection, err = net.Dial("tcp", config.Server)
		if err != nil {
			fmt.Printf("Cannot connect to the server (%s): %s", config.Server, err.Error())
			return
		}
	}

	serverConnection := NewTTYProtocolConn(rawConnection)
	reply, err := serverConnection.InitSender(SenderSessionInfo{
		Salt:              "salt",
		PasswordVerifierA: "PV_A",
	})

	if err != nil {
		fmt.Printf("Cannot initialise the protocol connection: %s", err.Error())
		return
	}

	log.Infof("Web terminal: %s", reply.URLWebReadWrite)

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	fmt.Printf("Web terminal: %s\n\n\r", reply.URLWebReadWrite)
	//TODO: if the user on the remote side presses keys, and so messages are sent back to the
	// tty sender, they will be delivered all at once, after Enter has been pressed. Fix that.

	ptyMaster := ptyMasterNew()
	ptyMaster.Start(config.commandName, strings.Fields(config.commandArgs), func(cols, rows int) {
		log.Infof("New window size: %dx%d", cols, rows)
		serverConnection.SetWinSize(cols, rows)
	})

	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		serverConnection.SetWinSize(cols, rows)
	}

	allWriter := io.MultiWriter(os.Stdout, serverConnection)

	go func() {
		_, err := io.Copy(allWriter, ptyMaster)
		if err != nil {
			log.Error("Lost connection with the server.\n")
			ptyMaster.Stop()
		}
	}()

	go func() {
		for {
			msg, err := serverConnection.ReadMessage()

			if err != nil {
				fmt.Printf(" -- Finishing the server connection with error: %s", err.Error())
				break
			}

			if msg.Type == MsgIDWrite {
				var msgWrite MsgTTYWrite
				json.Unmarshal(msg.Data, &msgWrite)
				ptyMaster.Write(msgWrite.Data[:msgWrite.Size])
			}
			if msg.Type == MsgIDSenderNewReceiverConnected {
				var msgReceiverConnected MsgTTYSenderNewReceiverConnected
				json.Unmarshal(msg.Data, &msgReceiverConnected)

				ptyMaster.Refresh()
				fmt.Printf("New receiver connected: %s ", msgReceiverConnected.Name)
			}
		}
	}()

	go func() {
		io.Copy(ptyMaster, os.Stdin)
	}()

	ptyMaster.Wait()
}
