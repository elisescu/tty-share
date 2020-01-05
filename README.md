[![Build Status](https://travis-ci.com/elisescu/tty-share.svg?branch=master)](https://travis-ci.com/elisescu/tty-share)

# tty-share

It is a very simple command line tool that gives remote access to a UNIX terminal session. It's using the [PTY](https://en.wikipedia.org/wiki/Pseudoterminal) system, so it should work on any *UNIX* system (Linux, OSX). Because it's written in GO, the tool will be a single binary, with no dependencies, which will also work on your ARM Raspberry Pi.

The most important part about it is that it requires **no setup** on the remote end. All I need to give remote access to the terminal (a bash/shell session) is the binary tool, and the remote person only needs to open a secret URL in their browser.

The project consists of two command line utilities: `tty-share` and `tty-server`. The server side has been moved to [github.com/elisescu/tty-server](https://github.com/elisescu/tty-server) repo, so it's easier to build the `tty-share` tool separately.

The `tty-share` is used on the machine that wants to share the terminal, and it connects to the server to generate a secret URL, over which the terminal can be viewed in the browser.

An instance of the server runs at [tty-share.com](https://tty-share.com), so you only need the `tty-server` binary if you want to host it yourself.

![demo](doc/demo.gif)

## More documentation

The documentation is very poor now. More will follow. [This](doc/architecture.md) describes briefly some thoughts behind the architecture of the project.

## Running

Download the latest `tty-share` binary [release](https://github.com/elisescu/tty-share/releases), and run it:

```
bash$ tty-share
Web terminal: https://go.tty-share.com/s/J5U6FAwChWNP0I9VQ9XyPqVD6m6IpI8-sBLRiz98XMA=

bash$
```

## Building `tty-share` locally

If you want to just build the tool that shares your terminal, and not the server, then simply do a

```
go get github.com/elisescu/tty-share
```

For cross-compilation you can use the GO building [environment variables](https://golang.org/doc/install/source#environment). For example, to build the `tty-share` for raspberrypi, you can do `GOOS=linux GOARCH=arm GOARM=6 make out/tty-share` (you can check your raspberrypi arch with `uname -a`).

## Security

`tty-share` connects over a TLS connection to the server, which uses a proxy for the SSL termination, and the browser terminal is served over HTTPS. The communication on both sides is encrypted and secured, in the same way as other similar tools are doing it (e.g. tmate, VSC, etc).

However, end-to-end encryption is still desired, so nothing but the sender and receiver can decrypt the data passed around.

## TODO

There are several improvements, and additions that can be done further:
  * Update and write more tests.
  * Add support for listening on sender connections over TLS.
  * React on the `tty-receiver` window size as well. For now, the size of the terminal window is decided by the `tty-share`, but perhaps both the sender and receiver should have a say in this.
  * Read only sessions, where the `tty_receiver` side can only watch, and cannot interact with the terminal session.
  * Command line `tty_receiver` can be implemented as well, without the need of a browser.
  * End-to-end encryption. Right now, the server can see what messages the sender and receiver are exchanging, but an end-to-end encryption layer can be built on top of this. It has been thought from the beginning, but it's just not implemented. The terminal IO messages are packed in protocol messages, and the payload can be easily encrypted with a shared key derived from a password that only the sender and receiver know.
  * Notify the `tty-share` user when a `tty-receiver` got connected (when the remote person opened the URL in their browser).
  * Many other


## Similar solutions

### VSC (Visual Studio Code) [Live Share](https://docs.microsoft.com/en-us/visualstudio/liveshare/use/vscode)

I've tried Visual Studio Code sharing, and it seems to work relatively well. One big advantage is that both persons in the session can write code, and navigate independently of each other. It also supports terminal sharing.

However, the two disadvantages with this tool are the need of logging in with a Github (or Microsoft) account, and having to install Visual Studio Code on the remote side too. I don't want to force the remote person to install VSC just for them to get access to a terminal session. Visual Studio Code might be popular in the web development circles, but it is not popular in the other development corners.

### [tmate.io](https://tmate.io/)

This is a great tool, and I used it quite a few times before. At the time I started my project, [tmate.io](https://tmate.io) didn't have the option to join the session from the browser, and one had to use `ssh`. In most cases, `ssh` is not a problem at all - in fact it's even preferred, but there are cases when you just don't have easy access to an `ssh` client, e.g.: joining from a Windows machine, or from your smartphone. In the meantime, the project added some support for joining a terminal session in the browser too.

Perhaps one downside with *tmate* is that it comes with quite a few dependencies which can make your life complicated if you want to compile it for ARM, for example. Running it on my raspberry pi might not be as simple as you want it, unless you use Debian.

## Credits

* [xterm.js](https://xtermjs.org/) - used in the browser receiver.
* [gotty](https://github.com/yudai/gotty) - used for inspiration and motivation.
* [tmate](https://tmate.io/) - inspiration and motivation.
* [https://codepen.io/chiaren/pen/ALwnI](https://codepen.io/chiaren/pen/ALwnI) - for the free 404 page.
