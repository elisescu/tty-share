# Old version

The old version of tty-share was working on a different architecture. The `tty-share` command was smaller, and the [tty-server](https://github.com/elisescu/tty-server) was serving the browser code. I decided to change that and make the server side simpler so it only forwards data back and forth, acting exactly like a reverse proxy. The only difference between `tty-proxy` and a reverse proxy, is that it doesn't create the connections to the target by itself, but instead it's the target (the `tty-share` command) that create the TCP connection over which then the connections are proxied to.

The two main reasons for this change are:
1. **non public sessions**: if you want to share a terminal session only in the local network, that is now possible. For this to work, the `tty-share` command has to contain all the browser code too.
2. **decoupling** between the public server and the `tty-share` so the user sharing is always in control of what code will end up running in the browser, without relying on the server. This makes backwards compatibility easier to maintain, at least for the browser sessions.

This change also means that:
* the [tty-server]() is now deprecated, and `tty-proxy` should be used instead. I reflected on whether I should have kept the same name or not, but I felt that using a different one might avoid confusion, given that they are two completely different applications.
* you will need to use the latest version of the `tty-share` that works with this new architecture.
