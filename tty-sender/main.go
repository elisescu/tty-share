package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/elisescu/tty-share/common"
	logrus "github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	commandName := flag.String("command", "bash", "The command to run")
	commandArgs := flag.String("args", "", "The command arguments")
	logFileName := flag.String("logfile", "-", "The name of the file to log")
	useTLS := flag.Bool("useTLS", true, "Use TLS to connect to the server")
	server := flag.String("server", "localhost:7654", "tty-server address")
	flag.Parse()

	log.Level = logrus.ErrorLevel
	if *logFileName != "-" {
		fmt.Printf("Writing logs to: %s\n", *logFileName)
		logFile, err := os.Create(*logFileName)
		if err != nil {
			fmt.Printf("Can't open %s for writing logs\n", *logFileName)
		}
		log.Level = logrus.DebugLevel
		log.Out = logFile
	}

	// TODO: check we are running inside a tty environment, and exit if not

	var rawConnection io.ReadWriteCloser
	if *useTLS {
		roots, err := x509.SystemCertPool()
		if err != nil {
			fmt.Printf("Cannot connect to the server (%s): %s", *server, err.Error())
			return
		}
		rawConnection, err = tls.Dial("tcp", *server, &tls.Config{RootCAs: roots})
		if err != nil {
			fmt.Printf("Cannot connect (TLS) to the server (%s): %s", *server, err.Error())
			return
		}
	} else {
		var err error
		rawConnection, err = net.Dial("tcp", *server)
		if err != nil {
			fmt.Printf("Cannot connect to the server (%s): %s", *server, err.Error())
			return
		}
	}

	serverConnection := common.NewTTYProtocolConn(rawConnection)
	reply, err := serverConnection.InitSender(common.SenderSessionInfo{
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
	// tty_sender, they will be delivered all at once, after Enter has been pressed. Fix that.

	ptyMaster := ptyMasterNew()
	ptyMaster.Start(*commandName, strings.Fields(*commandArgs), func(cols, rows int) {
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

			if msg.Type == common.MsgIDWrite {
				var msgWrite common.MsgTTYWrite
				json.Unmarshal(msg.Data, &msgWrite)
				ptyMaster.Write(msgWrite.Data[:msgWrite.Size])
			}
			if msg.Type == common.MsgIDSenderNewReceiverConnected {
				var msgReceiverConnected common.MsgTTYSenderNewReceiverConnected
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
