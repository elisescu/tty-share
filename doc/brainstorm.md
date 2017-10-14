## Overview of the architecture

```

                                    B
          A                        +-------------+
          +-----------------+      |             |       C
          |  TTYSender(cmd) | <+-> |  TTYProxy   |      +-------------------+
          +-----------------+      |   Server    | <-+->| TTYReceiver(web)  |
                                   |             |      +-------------------+
                                   |             |
                                   |             |       D
                                   |             |      +-------------------+
                                   |             | <-+->| TTYReceiver(ssh)  |
                                   |             |      +-------------------+
                                   |             |
          M                        |             |      N
          +-----------------+      |             |      +-------------------+
          |  TTYSender(cmd) | <+-> |             | <-+->| TTYREceiver(web)  |
          +-----------------+      +-------------+      +-------------------+
```
##
```
A <-> C, D
M <-> N
```

### A
Where A is the TTYSender, which will be used by the user Alice to share her terminal session. She will start it in the command line, with something like:
```
tty-share bash
```
If everything is successful, A will output to stdout 3 URLs, which, something like:
```
1. read-only: https://tty-share.io/s/0ENHQGjqaB
2. write:     https://tty-share.io/s/4HGFN8jahg
3. terminal:  ssh://0ENHQGjqaB@tty-share.io.com -p1234
4. admin:     http://localhost:5456/admin
```
Url number 1. will provide read-only access to the command shared. Which means the user will not be able to interact with the terminal.
Url number 2. will allow the user to interact with the terminale.
Url number 3. ssh access, to follow the remote command from a remote terminal.
Url number 4. provides an interface to control various options related to sharing.
### B
B is the TTYProxyServer, which will be publicly accessible and to which the TTYSender will connect to. On the TTYProxyServer will be created te sessions (read-only and write), and URLs will be returned back to A. Whent the command that A started exits, the session will end, so C should know.
### C
C is the browser via which user Chris will receive the terminal which Alice has shared.

### Corner cases
Corner cases to test for:
* AB connection cannot be done
* AB is established, but CB can't be done
* AB connection can go down
* CB connection can go down:
   - The websocket connection can go down
   - The browser refreshed. Command is still running, so the session is still valid
* All users from the C side close their connection
* The commmand finishes