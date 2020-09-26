# Proxy

`tty-share` will open a `TCP` connection to a public proxy.

This proxy will act like a reverse proxy, but it will not open new connections to any new back
server, and instead it will simulate multiple multiplexed connections over the `TCP` connection
already opened by `tty-share` mentioned above.

The proxy will be completely dumb. It will only proxy connections coming from client browsers to the
`tty-share`. It will be the `tty-share` command that will serve the content, and the ws
connections. Any SSL/WSS from the web clients will be terminated by nginx or other typical reverse
proxy.
