package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/elisescu/tty-share/proxy"
	"github.com/elisescu/tty-share/server"
	ttyServer "github.com/elisescu/tty-share/server"
	log "github.com/sirupsen/logrus"
)

var version string = "0.0.0"


func createServer(frontListenAddress string, frontendPath string, tty io.Writer, sessionID string) *server.TTYServer {
	config := ttyServer.TTYServerConfig{
		FrontListenAddress: frontListenAddress,
		FrontendPath:       frontendPath,
		TTYWriter:          tty,
		SessionID:          sessionID,
	}

	server := ttyServer.NewTTYServer(config)
	return server
}

type nilWriter struct {
}

func (nw *nilWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func main() {
	usageString := `
Usage:
  tty-share creates a session to a terminal application with remote participants. The session can be joined either from the browser, or by tty-share command itself.

    tty-share [flags]         ; share the terminal and get a session URL, as a server
    tty-share <session URL>   ; connect to an existing session, as a client

Examples:
  Start bash and create a public sharing session, so it's accessible outside the local network, and make the session read only:

    tty-share --public --readonly --command bash

  Join a remote session by providing the URL created another tty-share command:

     tty-share http://localhost:8000/local/

Flags:
`
	commandName := flag.String("command", os.Getenv("SHELL"), "The command to run")
	if *commandName == "" {
		*commandName = "bash"
	}
	commandArgs := flag.String("args", "", "The command arguments")
	logFileName := flag.String("logfile", "-", "The name of the file to log")
	listenAddress := flag.String("listen", "localhost:8000", "tty-server address")
	versionFlag := flag.Bool("version", false, "Print the tty-share version")
	frontendPath := flag.String("frontend-path", "", "The path to the frontend resources. By default, these resources are included in the server binary, so you only need this path if you don't want to use the bundled ones.")
	proxyServerAddress := flag.String("tty-proxy", "localhost:9000", "Address of the proxy for public facing connections")
	readOnly := flag.Bool("readonly", false, "Start a read only session")
	publicSession := flag.Bool("public", false, "Create a public session")
	noTLS := flag.Bool("no-tls", false, "Don't use TLS to connect to the tty-proxy server. Useful for local debugging")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s", usageString)
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
	}

	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s\n", version)
		return
	}

	// tty-share can work in two modes: either starting a command to be shared by acting as a
	// server, or by acting as a client for the remote side If we have an argument, that is not
	// a flag, passed to tty-share, we expect that to be the URl to connect to, as a
	// client. Otherwise, tty-share will act as the server.
	args := flag.Args()
	if len(args) == 1 {
		connectURL := args[0]
		client := newTtyShareClient(connectURL)

		err := client.Run()
		if err != nil {
			fmt.Printf("Cannot connect to the remote session: %s\n", err.Error())
		}
		return
	}

	log.SetLevel(log.InfoLevel)
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

	sessionID := "local"
	if *publicSession {
		proxy, err := proxy.NewProxyConnection(*listenAddress, *proxyServerAddress, *noTLS)
		if err != nil {
			fmt.Printf("Can't connect to the proxy: %s\n", err.Error())
			return
		}

		go proxy.RunProxy()
		sessionID = proxy.SessionID
		fmt.Printf("public session: %s\n", proxy.PublicURL)
		defer proxy.Stop()
	}

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	fmt.Printf("local session: http://%s/local/\n", *listenAddress)
	fmt.Printf("Press Enter to continue!\n")
	bufio.NewReader(os.Stdin).ReadString('\n')

	ptyMaster := ptyMasterNew()
	ptyMaster.Start(*commandName, strings.Fields(*commandArgs))

	var writer io.Writer = ptyMaster
	if *readOnly {
		writer = &nilWriter{}
	}

	server := createServer(*listenAddress, *frontendPath, writer, sessionID)
	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		server.WindowSize(cols, rows)
	}

	ptyMaster.SetWinChangeCB(func(cols, rows int) {
		log.Debugf("New window size: %dx%d", cols, rows)
		server.WindowSize(cols, rows)
	})

	mw := io.MultiWriter(os.Stdout, server)

	go func() {
		err := server.Run(func(clientAddr string) {
			ptyMaster.Refresh()
		})
		if err != nil {
			log.Error(err.Error())
		}
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
