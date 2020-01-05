// +build !windows

package pty

import (
	"os"
	"syscall"
	"unsafe"
)

// Getsize returns the number of rows (lines) and cols (positions
// in each line) in terminal t.
func Getsize(t *os.File) (rows, cols int, err error) {
	var ws winsize
	err = getwindowrect(&ws, t.Fd())
	return int(ws.ws_row), int(ws.ws_col), err
}

// Setsize sets the number of rows (lines) and cols (positions
// in each line) in terminal t. Both rows and cols have to be
// positive integers.
func Setsize(t *os.File, rows, cols int) error {
	ws := winsize{
		ws_col:    uint16(cols),
		ws_row:    uint16(rows),
		ws_xpixel: uint16(0), // not used
		ws_ypixel: uint16(0), // not used
	}

	return setwindowrect(&ws, t.Fd())
}

type winsize struct {
	ws_row    uint16
	ws_col    uint16
	ws_xpixel uint16
	ws_ypixel uint16
}

func getwindowrect(ws *winsize, fd uintptr) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

func setwindowrect(ws *winsize, fd uintptr) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
