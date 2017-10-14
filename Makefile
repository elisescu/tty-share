TTY_SERVER=tty_server
TTY_SENDER=tty_sender

TTY_SERVER_SRC=$(wildcard ./tty-server/*.go)
TTY_SENDER_SRC=$(wildcard ./tty-sender/*.go)
EXTRA_BUILD_DEPS=$(wildcard ./common/*go)

all: $(TTY_SERVER) $(TTY_SENDER)
	@echo "All done"

$(TTY_SERVER): $(TTY_SERVER_SRC) $(EXTRA_BUILD_DEPS)
	go build -o $@ $(TTY_SERVER_SRC)

$(TTY_SENDER): $(TTY_SENDER_SRC) $(EXTRA_BUILD_DEPS)
	go build -o $@ $(TTY_SENDER_SRC)

frontend: FORCE
	cd frontend && npm run build && cd -

clean:
	@rm -f $(TTY_SERVER) $(TTY_SENDER)
	@echo "Cleaned"

runs: $(TTY_SERVER)
	@./$(TTY_SERVER) --url http://localhost:9090 --web_address :9090 --sender_address :7654

runc: $(TTY_SENDER)
	@./$(TTY_SENDER) --logfile output.log

test:
	@go test github.com/elisescu/tty-share/testing -v

FORCE:
