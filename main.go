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

func createServer(frontListenAddress string, frontendPath string, pty server.PTYHandler, sessionID string, allowTunneling bool, crossOrigin bool, baseUrlPath string) *server.TTYServer {
	config := ttyServer.TTYServerConfig{
		FrontListenAddress: frontListenAddress,
		FrontendPath:       frontendPath,
		PTY:                pty,
		SessionID:          sessionID,
		AllowTunneling:     allowTunneling,
		CrossOrigin:        crossOrigin,
		BaseUrlPath:        baseUrlPath,
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
      tty-share [--verbose] [--logfile <file name>] [-L <local_port>:<remote_host>:<remote_port>]
                [--detach-keys]                     <session URL>                 # connect to an existing session, as a client

Examples:
  Start bash and create a public sharing session, so it's accessible outside the local network, and make the session read only:

      tty-share --public --readonly --command bash

  Join a remote session by providing the URL created another tty-share command:

      tty-share http://localhost:8000/s/local/

Flags:
[c] - flags that are used only by the client
[s] - flags that are used only by the server
`
	commandName := flag.String("command", os.Getenv("SHELL"), "[s] The command to run")
	if *commandName == "" {
		*commandName = "bash"
	}
	commandArgs := flag.String("args", "", "[s] The command arguments")
	logFileName := flag.String("logfile", "-", "The name of the file to log")
	listenAddress := flag.String("listen", "localhost:8000", "[s] tty-server address")
	versionFlag := flag.Bool("version", false, "Print the tty-share version")
	frontendPath := flag.String("frontend-path", "", "[s] The path to the frontend resources. By default, these resources are included in the server binary, so you only need this path if you don't want to use the bundled ones.")
	proxyServerAddress := flag.String("tty-proxy", "on.tty-share.com:4567", "[s] Address of the proxy for public facing connections")
	readOnly := flag.Bool("readonly", false, "[s] Start a read only session")
	publicSession := flag.Bool("public", false, "[s] Create a public session")
	noTLS := flag.Bool("no-tls", false, "[s] Don't use TLS to connect to the tty-proxy server. Useful for local debugging")
	noWaitEnter := flag.Bool("no-wait", false, "[s] Don't wait for the Enter press before starting the session")
	headless := flag.Bool("headless", false, "[s] Don't expect an interactive terminal at stdin")
	headlessCols := flag.Int("headless-cols", 80, "[s] Number of cols for the allocated pty when running headless")
	headlessRows := flag.Int("headless-rows", 25, "[s] Number of rows for the allocated pty when running headless")
	detachKeys := flag.String("detach-keys", "ctrl-o,ctrl-c", "[c] Sequence of keys to press for closing the connection. Supported: https://godoc.org/github.com/moby/term#pkg-variables.")
	allowTunneling := flag.Bool("A", false, "[s] Allow clients to create a TCP tunnel")
	tunnelConfig := flag.String("L", "", "[c] TCP tunneling addresses: local_port:remote_host:remote_port. The client will listen on local_port for TCP connections, and will forward those to the from the server side to remote_host:remote_port")
	crossOrgin := flag.Bool("cross-origin", false, "[s] Allow cross origin requests to the server")
	baseUrlPath := flag.String("base-url-path", "", "[s] The base URL path on the serve")

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

		client := newTtyShareClient(connectURL, *detachKeys, tunnelConfig)

		err := client.Run()
		if err != nil {
			fmt.Printf("Cannot connect to the remote session. Make sure the URL points to a valid tty-share session.\n")
		}
		fmt.Printf("\ntty-share disconnected\n\n")
		return
	}

	// tty-share works as a server, from here on
	if !isStdinTerminal() && !*headless {
		fmt.Printf("Input not a tty\n")
		os.Exit(1)
	}

	sessionID := ""
	publicURL := ""
	if *publicSession {
		proxy, err := proxy.NewProxyConnection(*listenAddress, *proxyServerAddress, *noTLS)
		if err != nil {
			log.Errorf("Can't connect to the proxy: %s\n", err.Error())
			return
		}

		go proxy.RunProxy()
		sessionID = proxy.SessionID
		publicURL = proxy.PublicURL
		defer proxy.Stop()
	}

	envVars := os.Environ()
	envVars = append(envVars,
		fmt.Sprintf("TTY_SHARE_LOCAL_URL=http://%s", *listenAddress),
		fmt.Sprintf("TTY_SHARE=1", os.Getpid()),
	)

	if publicURL != "" {
		envVars = append(envVars,
			fmt.Sprintf("TTY_SHARE_PUBLIC_URL=%s", publicURL),
		)
	}

	ptyMaster := ptyMasterNew(*headless, *headlessCols, *headlessRows)
	err := ptyMaster.Start(*commandName, strings.Fields(*commandArgs), envVars)
	if err != nil {
		log.Errorf("Cannot start the %s command: %s", *commandName, err.Error())
		return
	}

	// Display the session information to the user, before showing any output from the command.
	// Wait until the user presses Enter
	if publicURL != "" {
		fmt.Printf("public session: %s\n", publicURL)
	}

	// Ensure the base URL path does not end with a forward slash,
	// and that there are no excessive forward slashes at the beginning.
	// A base URL of "/" will be trimmed to an empty string.
	sanitizedBaseUrlPath := strings.Trim(*baseUrlPath, "/")
	if sanitizedBaseUrlPath != "" {
		sanitizedBaseUrlPath = "/" + sanitizedBaseUrlPath
	}

	fmt.Printf("local session: http://%s%s/s/local/\n", *listenAddress, sanitizedBaseUrlPath)

	if !*noWaitEnter && !*headless {
		fmt.Printf("Press Enter to continue!\n")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}

	stopPtyAndRestore := func() {
		ptyMaster.Stop()
		ptyMaster.Restore()
	}

	ptyMaster.MakeRaw()
	defer stopPtyAndRestore()
	var pty server.PTYHandler = ptyMaster
	if *readOnly {
		pty = &nilPTY{}
	}

	server := createServer(*listenAddress, *frontendPath, pty, sessionID, *allowTunneling, *crossOrgin, sanitizedBaseUrlPath)
	if cols, rows, e := ptyMaster.GetWinSize(); e == nil {
		server.WindowSize(cols, rows)
	}

	ptyMaster.SetWinChangeCB(func(cols, rows int) {
		log.Debugf("New window size: %dx%d", cols, rows)
		server.WindowSize(cols, rows)
	})

	var mw io.Writer
	mw = server
	if !*headless {
		mw = io.MultiWriter(os.Stdout, server)
	}

	go func() {
		err := server.Run()
		if err != nil {
			stopPtyAndRestore()
			log.Errorf("Server finished: %s", err.Error())
		}
	}()

	go func() {
		_, err := io.Copy(mw, ptyMaster)
		if err != nil {
			stopPtyAndRestore()
		}
	}()

	if !*headless {
		go func() {
			_, err := io.Copy(ptyMaster, os.Stdin)
			if err != nil {
				stopPtyAndRestore()
			}
		}()
	}

	ptyMaster.Wait()
	fmt.Printf("tty-share finished\n\n\r")
	server.Stop()
}
