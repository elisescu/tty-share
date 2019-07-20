High level architecture
=======================


```
     Alice

+-tty-share---------------+              +-tty-server----------------+                 Bob
|                         |              |                           | https://    +------------+
| +------+       +-----+  |     TLS      | +------+     +---------+  | wss://      |tty-receiver|
| | bash | <-+-> |proto| <---------------> | proto| <-> | session | +-----+------> |     1      |
| +------+   |   +-----+  |              | +------+     +---------+  |    |        +------------+
|            |            |              |                           |    |
|            +-> pty      |              +---------------------------+    |        +------------+
+-------------------------+                                               +------> |tty-receiver|
                                                                                   |     2      |
                                                                                   +------------+
```

Alice wants to share a terminal session with Bob, so she starts `tty-share` on her machine, inside the terminal. `tty-share` then connects to the `tty-server` and starts inside a `bash` process. It then puts the terminal in which it was started in RAW mode, and the stdin and stdout are multiplexed to/from the `bash` process it started, and the remote connection to the `tty-server`. On the server side, a session is created, which connects the `tty-share` connection with the future `tty-receiver` instances, running in the browser. The `tty-receiver` runs inside the browser, on top of [xterm.js](https://xtermjs.org/), and is served by the server, via a unique session URL. Alice has to send this unique URL to Bob.

Once the connection is established, Bob can then interact inside the browser with the `bash` session started by Alice. When Bob presses, for example, the key `a`, this is detected by `xterm.js` and sent via a websockets connection to the server side. From there, it is sent to the `tty-share` which sends it to the pseudo terminal attached to the `bash` process started inside `tty-share`. Then character `a` is received via the standard output of the `bash` command, and is sent from there both to the standard output of the `tty-share`, so Alice can see it, and also to the `tty-receiver` (via the server), so Bob can see it too.

More specific details on how this is implemented, can be seen in the source code of the `tty-share`.
