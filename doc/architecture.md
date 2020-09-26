# High level flow

## Direct terminal sharing

No proxy needed. `tty-share` will start a command, and be ready to serve it's output and input over WS connections.

## Proxy terminal sharing

- the `tty-share` opens a TCP connection to the `tty-proxy`
- the `tty-proxy` proxy accepts the connection, generates a session ID, and sends it back to `tty-share`
  - there is now a direct connection between the two
  - a session ID to map any web connections to the `tty-share` side

- `tty-share` gets http requests:
  - `/` (direct, from a listening server): serves the `index.html` - templated for direct requests (e..g: `<script src="/static/tty-receiver.js"></script>`)
  - `/` (via the `tty-proxy` TCP connection): serves the `index.html` - templated for the respective session (it already has the session). (e.g.: `<script src="<id>/static/tty-receiver.js"></script>`)
  - `/ws/` (direct, from the listening server): accepts a WS connection and connects it to the command opened
  - `/ws/` (via the `tty-proxy` TCP connection): accepts a WS connection and connects it to the command opened

- `tty-proxy` gets HTTP requests:
  - `/s/<id>/*` - builds a HTTP request forwards it to the TCP connection it has to the `tty-share` for that `<id>`
  - `/ws/<id>` - forwards the WS connection to the `tty-share`, over the *same* TCP connection as above

- over the same TCP connection, we have to pass multiple requests + a WS connection
  - https://godoc.org/github.com/hashicorp/yamux
  - https://godoc.org/github.com/soheilhy/cmux#pkg-examples

x
