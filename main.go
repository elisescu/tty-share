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

func createServer(frontListenAddress string, frontendPath string, pty server.PTYHandler, sessionID string) *server.TTYServer {
	config := ttyServer.TTYServerConfig{
		FrontListenAddress: frontListenAddress,
		FrontendPath:       frontendPath,
		PTY:                pty,
		SessionID:          sessionID,
	}

	server := ttyServer.NewTTYServer(config)
	return server
}

type nilPTY struct {
}

func (nw *nilPTY) Write(data []byte) (int, error) {
	return len(data), nil
}

func (nw *nilPTY) Refresh() {
}

func main() {
	usageString := `
Usage:
  tty-share creates a session to a terminal application with remote participants. The session can be joined either from the browser, or by tty-share command itself.

      tty-share [[--args <"args">] --command <executable>]                        # share the terminal and get a session URL, as a server
                [--logfile <file name>] [--listen <[ip]:port>]
                [--frontend-path <path>] [--tty-proxy <host:port>]
                [--readonly] [--public] [no-tls] [--verbose] [--version]
      tty-share [--verbose] [--logfile <file name>]
                [--detach-keys]                     <session URL>                 # connect to an existing session, as a client

Examples:
  Start bash and create a public sharing session, so it's accessible outside the local network, and make the session read only:

      tty-share --public --readonly --command bash

  Join a remote session by providing the URL created another tty-share command:

      tty-share http://localhost:8000/s/local/

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
	proxyServerAddress := flag.String("tty-proxy", "on.tty-share.com:4567", "Address of the proxy for public facing connections")
	readOnly := flag.Bool("readonly", false, "Start a read only session")
	publicSession := flag.Bool("public", false, "Create a public session")
	noTLS := flag.Bool("no-tls", false, "Don't use TLS to connect to the tty-proxy server. Useful for local debugging")
	detachKeys := flag.String("detach-keys", "ctrl-o,ctrl-c", "Sequence of keys to press for closing the connection. Supported: https://godoc.org/github.com/moby/term#pkg-variables.")
	verbose := flag.Bool("verbose", false, "Verbose logging")
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

	// Log setup
	log.SetLevel(log.WarnLevel)
	if *verbose {
		log.SetLevel(log.DebugLevel)
	}

	if *logFileName != "-" {
		logFile, err := os.Create(*logFileName)
		if err != nil {
			fmt.Printf("Can't open %s for writing logs\n", *logFileName)
		}
		log.SetOutput(logFile)
	}

	// tty-share can work in two modes: either starting a command to be shared by acting as a
	// server, or by acting as a client for the remote side If we have an argument, that is not
	// a flag, passed to tty-share, we expect that to be the URl to connect to, as a
	// client. Otherwise, tty-share will act as the server.
	args := flag.Args()
	if len(args) == 1 {
		connectURL := args[0]
		client := newTtyShareClient(connectURL, *detachKeys)

		err := client.Run()
		if err != nil {
			fmt.Printf("Cannot connect to the remote session. Make sure the URL points to a valid tty-share session.\n")
		}
		fmt.Printf("\ntty-share disconnected\n\n")
		return
	}

	// tty-share works as a server, from here on
	if !isStdinTerminal() {
		fmt.Printf("Input not a tty\n")
		os.Exit(1)
	}

	sessionID := "local"
	if *publicSession {
		proxy, err := proxy.NewProxyConnection(*listenAddress, *proxyServerAddress, *noTLS)
		if err != nil {
			log.Errorf("Can't connect to the proxy: %s\n", err.Error())
			return
		}

		go proxy.RunProxy()
		sessionID = proxy.SessionID
		fmt.Printf("public session: %s\n", proxy.PublicURL)
		defer proxy.Stop()
	}

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	fmt.Printf("local session: http://%s/s/local/\n", *listenAddress)
	fmt.Printf("Press Enter to continue!\n")
	bufio.NewReader(os.Stdin).ReadString('\n')

	ptyMaster := ptyMasterNew()
	defer ptyMaster.Restore()

	ptyMaster.Start(*commandName, strings.Fields(*commandArgs))

	var pty server.PTYHandler = ptyMaster
	if *readOnly {
		pty = &nilPTY{}
	}

	server := createServer(*listenAddress, *frontendPath, pty, sessionID)
	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		server.WindowSize(cols, rows)
	}

	ptyMaster.SetWinChangeCB(func(cols, rows int) {
		log.Debugf("New window size: %dx%d", cols, rows)
		server.WindowSize(cols, rows)
	})

	mw := io.MultiWriter(os.Stdout, server)

	go func() {
		err := server.Run()
		if err != nil {
			log.Debugf("Server done: %s", err.Error())
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
	fmt.Printf("tty-share finished\n\n")
	server.Stop()
}
