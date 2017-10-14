- process reads input
- key shortcuts . same as input?
- process writes to output?

Notes:
- background programs are suspended when they try to run to the terminal
- user input redirected to foreground program only
- UART driver, line discipline instance and TTY driver compose a TTY device
- job = process group (jobs, fg, bg)
- session, session leader (shell - talks to the kernel through signals and system calls)

```
cat &
ls | sort
```
As you can see in the diagram above, several processes have /dev/pts/0 attached to their standard input. 
But only the foreground job (the ls | sort pipeline) will receive input from the TTY.  
Likewise, only the foreground job will be allowed to write to the TTY device (in the default configuration). 
If the cat process were to attempt to write to the TTY, the kernel would suspend it using a signal.


End-to-end encryption:
======================
* sender:
    - generate salt, and the shared key from the password
    - connect to the server sending the session start info:
        SessionStart {
            salt String
            passwordVerifierA String
            passwordVerifierB String
            allowSSHNotEndToEnd bool
        }
    - gets one of the two replies:
        SessionStartNotOk - should stop; or
        SessionStartOK {
            webTTYUrl string
            sshTTYUrl string
        }

* web-receiver:
    - open the link: get the html page and try to open the WS connection.
    - has the same salt as the sender, served in the web page
    - asks the user for the password and checks the password and asks server to validate password verifier
* ssh-receiver:
    - connects with ssh to some-random-string@tty-share.io