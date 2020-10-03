package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	ptyDevice "github.com/elisescu/pty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type onWindowChangedCB func(int, int)

// This defines a PTY Master whih will encapsulate the command we want to run, and provide simple
// access to the command, to write and read IO, but also to control the window size.
type ptyMaster struct {
	ptyFile           *os.File
	command           *exec.Cmd
	terminalInitState *terminal.State
}

func ptyMasterNew() *ptyMaster {
	return &ptyMaster{}
}

func isStdinTerminal() bool {
	return terminal.IsTerminal(0)
}

func (pty *ptyMaster) Start(command string, args []string) (err error) {
	// Save the initial state of the terminal, before making it RAW. Note that this terminal is the
	// terminal under which the tty-share command has been started, and it's identified via the
	// stdin file descriptor (0 in this case)
	// We need to make this terminal RAW so that when the command (passed here as a string, a shell
	// usually), is receiving all the input, including the special characters:
	// so no SIGINT for Ctrl-C, but the RAW character data, so no line discipline.
	// Read more here: https://www.linusakesson.net/programming/tty/
	pty.terminalInitState, err = terminal.MakeRaw(0)

	pty.command = exec.Command(command, args...)
	pty.ptyFile, err = ptyDevice.Start(pty.command)

	if err != nil {
		return
	}

	// Set the initial window size
	cols, rows, err := terminal.GetSize(0)
	pty.SetWinSize(rows, cols)
	return
}

func (pty *ptyMaster) SetWinChangeCB(winChangedCB onWindowChangedCB) {
	// Start listening for window changes
	go onWindowChanges(func(cols, rows int) {
		// TODO:policy: should the server decide here if we care about the size and set it
		// right here?
		pty.SetWinSize(rows, cols)

		// Notify the ptyMaster user of the window changes, to be sent to the remote side
		winChangedCB(cols, rows)
	})
}

func (pty *ptyMaster) GetWinSize() (int, int, error) {
	cols, rows, err := terminal.GetSize(0)
	return cols, rows, err
}

func (pty *ptyMaster) Write(b []byte) (int, error) {
	return pty.ptyFile.Write(b)
}

func (pty *ptyMaster) Read(b []byte) (int, error) {
	return pty.ptyFile.Read(b)
}

func (pty *ptyMaster) SetWinSize(rows, cols int) {
	ptyDevice.Setsize(pty.ptyFile, rows, cols)
}

func (pty *ptyMaster) Refresh() {
	// We wanna force the app to re-draw itself, but there doesn't seem to be a way to do that
	// so we fake it by resizing the window quickly, making it smaller and then back big
	cols, rows, err := pty.GetWinSize()

	if err != nil {
		return
	}

	pty.SetWinSize(rows-1, cols)

	go func() {
		time.Sleep(time.Millisecond * 50)
		pty.SetWinSize(rows, cols)
	}()
}

func (pty *ptyMaster) Wait() (err error) {
	err = pty.command.Wait()
	// The terminal has to be restored from the RAW state, to its initial state
	terminal.Restore(0, pty.terminalInitState)
	return
}

func (pty *ptyMaster) Stop() (err error) {
	signal.Ignore(syscall.SIGWINCH)

	pty.command.Process.Signal(syscall.SIGTERM)
	// TODO: Find a proper wai to close the running command. Perhaps have a timeout after which,
	// if the command hasn't reacted to SIGTERM, then send a SIGKILL
	// (bash for example doesn't finish if only a SIGTERM has been sent)
	pty.command.Process.Signal(syscall.SIGKILL)
	return
}

func onWindowChanges(wcCB onWindowChangedCB) {
	wcChan := make(chan os.Signal, 1)
	signal.Notify(wcChan, syscall.SIGWINCH)
	// The interface for getting window changes from the pty slave to its process, is via signals.
	// In our case here, the tty-share command (built in this project) is the client, which should
	// get notified if the terminal window in which it runs has changed. To get that, it needs to
	// register for SIGWINCH signal, which is used by the kernel to tell process that the window
	// has changed its dimentions.
	// Read more here: https://www.linusakesson.net/programming/tty/
	// Shortly, ioctl calls are used to communicate from the process to the pty slave device,
	// and signals are used for the communiation in the reverse direction: from the pty slave
	// device to the process.

	for {
		select {
		case <-wcChan:
			cols, rows, err := terminal.GetSize(0)
			if err == nil {
				wcCB(cols, rows)
			} else {
				log.Warnf("Can't get window size: %s", err.Error())
			}
		}
	}
}
