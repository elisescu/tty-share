package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"strings"

	"github.com/elisescu/tty-share/server"
	ttyServer "github.com/elisescu/tty-share/server"
	log "github.com/sirupsen/logrus"
)

var version string = "0.0.0"

func createServer(frontListenAddress string, frontendPath string, tty io.Writer) *server.TTYServer {
	config := ttyServer.TTYServerConfig{
		FrontListenAddress: frontListenAddress,
		FrontendPath:       frontendPath,
		TTYWriter:          tty,
	}

	server := ttyServer.NewTTYServer(config)
	log.Info("Listening on address: http://", config.FrontListenAddress)
	return server
}

func main() {
	commandName := flag.String("command", os.Getenv("SHELL"), "The command to run")
	if *commandName == "" {
		*commandName = "bash"
	}
	commandArgs := flag.String("args", "", "The command arguments")
	logFileName := flag.String("logfile", "-", "The name of the file to log")
	listenAddress := flag.String("listen", "localhost:8080", "tty-server address")
	versionFlag := flag.Bool("version", false, "Print the tty-share version")
	frontendPath := flag.String("frontend_path", "", "The path to the frontend resources. By default, these resources are included in the server binary, so you only need this path if you don't want to use the bundled ones.")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s\n", version)
		return
	}

	log.SetLevel(log.ErrorLevel)
	if *logFileName != "-" {
		fmt.Printf("Writing logs to: %s\n", *logFileName)
		logFile, err := os.Create(*logFileName)
		if err != nil {
			fmt.Printf("Can't open %s for writing logs\n", *logFileName)
		}
		log.SetLevel(log.DebugLevel)
		log.SetOutput(logFile)
	}

	if !isStdinTerminal() {
		fmt.Printf("Input not a tty\n")
		os.Exit(1)
	}

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	fmt.Printf("Web terminal: http://%s\n\n\rPress Enter to continue!\n", *listenAddress)
	bufio.NewReader(os.Stdin).ReadString('\n')

	ptyMaster := ptyMasterNew()
	ptyMaster.Start(*commandName, strings.Fields(*commandArgs))

	server := createServer(*listenAddress, *frontendPath, ptyMaster)
	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		server.WindowSize(cols, rows)
	}

	ptyMaster.SetWinChangeCB(func(cols, rows int) {
		log.Infof("New window size: %dx%d", cols, rows)
		server.WindowSize(cols, rows)
	})

	mw := io.MultiWriter(os.Stdout, server)

	go func() {
		server.Run(func (clientAddr string){
			ptyMaster.Refresh()
		})
	}()

	go func() {
		_, err := io.Copy(mw, ptyMaster)
		if err != nil {
			log.Error("Lost connection with the server.\n")
			ptyMaster.Stop()
		}
	}()

	go func() {
		io.Copy(ptyMaster, os.Stdin)
	}()

	ptyMaster.Wait()
	fmt.Printf("tty-share finished.\n\r")
	server.Stop()
}
