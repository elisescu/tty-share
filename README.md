TTY Share
=========

A small tool that allows sharing the terminal over the Internet, in the web browser. It works with a shell command, or any other utility that relies on the unix PTY architecture.

It consists of two command line utilities: `tty_sender` and `tty_server`. The server is only needed if you want to host it yourself.

The `tty_sender` is used on the machine that wants to share the terminal, and it connects to the server to generate a unique URL, over which the terminal can be viewed in the browser.

Read more about how it works in the [documentation](doc/architecture.md).

![demo](doc/demo.gif)

Building and running the code
=============================

For an easy deployment, the server can bundle inside the binary all the frontend resources, so in the end, there will be only one file to be copied and deployed. However, the frontend resources, can also be served from a local folder, with a command line flag.

### Build all
``` 
cd frontend
npm install
npm run build # builds the frontend
cd -
make all # builds both the sender and server
```

### Run the server
```
make runs
```
Will run the server on the localhost.


### Run the sender
```
make runc
```
Will run the sender and connect it to the server running on the local host (so the above command has
to be ran first).

For more info, on how to run, see the Makefile, or the help of the two binaries (`tty_sender` and `tty_receiver`)

The project didn't follow the typical way of building go applications, because everything is put in one single project and package, for the ease of development, and also because the bundle containing the frontend has to be built as well. Perhaps there's a better way, but this works for now.


TLS and HTTPS
=============

At the moment the `tty_sender` supports connecting over a TLS connection to the server, but the
server doesn't have that implemented yet. However, the server can easily run behind a proxy which
can take care of encrypting the connections from the senders and receivers (doing both TLS and
HTTPS), without the server knowing about it.
However, the server should have support for being able to listen on TLS connections from the sender
as well, and this will be added in the future.

TODO
====

There are several improvements, and additions that can be done further:
* Update the tests and write some more.
* Add support for listening on sender connections over TLS.
* React on the `tty_receiver` window size as well. For now, the size of the terminal window is
  decided by the `tty_sender`, but perhaps both the sender and receiver should have a say in this.
* Read only sessions, where the `tty_reciver` side can only watch, and cannot interact with the
  terminal session on the sender side.
* Non-web based `tty_receiver` can be implemented as well, without the need of a browser, but using
  it from the command line.
* End-to-end encryption. Right now, the server can see what messages the sender and receiver are
  exchanging, but an end-to-end encryption layer can be built on top of this. It has been thought
  from the beginning, but it's just not implemented. The terminal IO messages are packed in protocol
  messages, and the payload can be easily encrypted with a shared key derived from a password that
  only the sender and receiver sides know.
 * Notify the `tty_sender` user when a `tty_receiver` got connected.
 * Many other

Other solutions
==============

* [tmate](https://tmate.io/) - is a very similar solution, which I have been using several times. It
  works really well - the only disadvantage, being that it doesn't support sharing it via the
  browser.

Credits
=======

* [xterm.js](https://xtermjs.org/) - used in the browser receiver.
* [gotty](https://github.com/yudai/gotty) - used for inspiration and motivation.
* [tmate](https://tmate.io/) - inspiration and motivation.
* [https://codepen.io/chiaren/pen/ALwnI](https://codepen.io/chiaren/pen/ALwnI) - for the free 404
  page.
