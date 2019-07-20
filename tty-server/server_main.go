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
	webAddress := flag.String("web_address", ":80", "The bind address for the web interface")
	senderAddress := flag.String("sender_address", ":6543", "The bind address for the tty-share connections")
	url := flag.String("url", "http://localhost", "The public web URL the server will be accessible at")
	frontendPath := flag.String("frontend_path", "", "The path to the frontend resources")
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
