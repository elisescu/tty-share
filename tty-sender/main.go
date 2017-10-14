package main

import (
	"bufio"
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
	server := flag.String("server", "localhost:7654", "tty-proxyserver address")
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

	tcpConn, err := net.Dial("tcp", *server)
	if err != nil {
		fmt.Printf("Cannot connect to the server (%s): %s", *server, err.Error())
		return
	}

	serverConnection := common.NewTTYProtocolConn(tcpConn)
	reply, err := serverConnection.InitSender(common.SenderSessionInfo{
		Salt:              "salt",
		PasswordVerifierA: "PV_A",
	})

	log.Infof("Web terminal: %s", reply.URLWebReadWrite)

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	fmt.Printf("Web terminal: %s. Press Enter to continue. \n\r", reply.URLWebReadWrite)
	bufio.NewReader(os.Stdin).ReadBytes('\n')
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
		}
	}()

	go func() {
		io.Copy(ptyMaster, os.Stdin)
	}()

	ptyMaster.Wait()
}
