package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	hcl "github.com/hashicorp/hcl"
)

// TTYShareConfig - configuration options for CLI flags or config file
type TTYShareConfig struct {
	LogFileName    string `json:"logFileName"`
	UseTLS         bool   `json:"useTLS"`
	Server         string `json:"server"`
	configFileName string
	commandName    string
	commandArgs    string
	versionFlag    bool
}

// Load - Load configuration from config file if available and parse CLI options
// CLI options override any config file settings, having config file is useful
// for changing default options
func (conf *TTYShareConfig) Load() {
	defConfigFileName := os.Getenv("HOME") + "/." + path.Base(os.Args[0])
	flag.StringVar(&conf.configFileName, "config", defConfigFileName, "Config file path")
	flag.StringVar(&conf.commandName, "command", os.Getenv("SHELL"), "The command to run")
	if conf.commandName == "" {
		conf.commandName = "bash"
	}
	flag.StringVar(&conf.commandArgs, "args", "", "The command arguments")
	flag.BoolVar(&conf.versionFlag, "version", false, "Print the tty-share version")
	flag.StringVar(&config.LogFileName, "logfile", config.LogFileName, "The name of the file to log")
	flag.BoolVar(&config.UseTLS, "useTLS", config.UseTLS, "Use TLS to connect to the server")
	flag.StringVar(&config.Server, "server", config.Server, "tty-server address")
	flag.Parse()

	if conf.versionFlag {
		fmt.Printf("%s\n", version)
		os.Exit(0)
	}

	if fileInfo, err := os.Stat(conf.configFileName); os.IsNotExist(err) {
		// Ignore if default config file does not exist
		// but report error otherwise
		if conf.configFileName != defConfigFileName {
			fmt.Fprintf(os.Stderr, "Config failed: %s\n", err.Error())
			os.Exit(1)
		}
	} else {
		var data []byte
		if fileInfo.Size() > 0x100000 {
			// Config larger than 1MiB makes no sense for this little program, report and exit
			fmt.Fprintf(os.Stderr, "Config failed: config file '%s' is too big\n", conf.configFileName)
			os.Exit(3)
		}
		if data, err = ioutil.ReadFile(conf.configFileName); err != nil {
			fmt.Fprintf(os.Stderr, "Config failed: %s\n", err.Error())
			os.Exit(2)
		}
		if err = hcl.Decode(conf, string(data)); err != nil {
			fmt.Fprintf(os.Stderr, "Config failed: %s\n", err.Error())
			os.Exit(4)
		}
		fmt.Printf("Configuration loaded from '%s'\n\n", conf.configFileName)
	}
	// Override config file options with CLI / give priority to CLI
	flag.Parse()
}
