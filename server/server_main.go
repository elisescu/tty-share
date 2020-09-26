package main

import (
	"flag"
	"os"
	"os/signal"

	logrus "github.com/sirupsen/logrus"
)

// MainLogger is the logger that will be used across the whole main package. I whish I knew of a better way
var MainLogger = logrus.New()

func main() {
	webAddress := flag.String("web_address", ":80", "The bind address for the web interface. This is the listening address for the web server that hosts the \"browser terminal\". You might want to change this if you don't want to use the port 80, or only bind the localhost.")
	senderAddress := flag.String("sender_address", ":6543", "The bind address for the tty-share TLS connections. tty-share tool will connect to this address.")
	url := flag.String("url", "http://localhost", "The public web URL the server will be accessible at. This will be sent back to the tty-share tool to display it to the user.")
	frontendPath := flag.String("frontend_path", "", "The path to the frontend resources. By default, these resources are included in the server binary, so you only need this path if you don't want to use the bundled ones.")
	flag.Parse()

	log := MainLogger
	log.SetLevel(logrus.DebugLevel)

	config := TTYServerConfig{
		WebAddress:       *webAddress,
		TTYSenderAddress: *senderAddress,
		ServerURL:        *url,
		FrontendPath:     *frontendPath,
	}

	server := NewTTYServer(config)

	// Install a signal and wait until we get Ctrl-C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		s := <-c
		log.Debug("Received signal <", s, ">. Stopping the server")
		server.Stop()
	}()

	log.Info("Listening on address: http://", config.WebAddress, ", and TCP://", config.TTYSenderAddress)
	err := server.Listen()

	log.Debug("Exiting. Error: ", err)
}
