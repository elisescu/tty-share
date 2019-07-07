TTY_SERVER=./out/tty-server
TTY_SENDER=./out/tty-sender

# We need to make sure the assets_bundle is in the list only onces in both these two special cases:
# a) first time, when the assets_bundle.go is generated, and b) when it's already existing there,
# but it has to be re-generated.
# Unfortunately, the assets_bundle.go seems to have to be in the same folder as the rest of the
# server sources, so that's why all this mess
TTY_SERVER_SRC=$(filter-out ./tty-server/assets_bundle.go, $(wildcard ./tty-server/*.go)) ./tty-server/assets_bundle.go
TTY_SENDER_SRC=$(wildcard ./tty-sender/*.go)
EXTRA_BUILD_DEPS=$(wildcard ./common/*go)
TTY_SERVER_ASSETS=$(wildcard frontend/public/*)

all: $(TTY_SERVER) $(TTY_SENDER)
	@echo "All done"

$(TTY_SERVER): $(TTY_SERVER_SRC) $(EXTRA_BUILD_DEPS)
	go build -o $@ $(TTY_SERVER_SRC)

$(TTY_SENDER): $(TTY_SENDER_SRC) $(EXTRA_BUILD_DEPS)
	go build -o $@ $(TTY_SENDER_SRC)

tty-server/assets_bundle.go: $(TTY_SERVER_ASSETS)
	go-bindata --prefix frontend/public/ -o $@ $^

# We only care about creating release binaries for the sender side
dist: frontend ./out/tty-sender.linux.zip ./out/tty-sender.osx.zip

%.zip: %
	zip $@ $^

./out/tty-sender.osx: $(TTY_SENDER_SRC) $(EXTRA_BUILD_DEPS)
	GOOS=darwin go build -o $@ $(TTY_SENDER_SRC)

./out/tty-sender.linux: $(TTY_SENDER_SRC) $(EXTRA_BUILD_DEPS)
	GOOS=linux go build -o $@ $(TTY_SENDER_SRC)

frontend: FORCE
	cd frontend && npm run build && cd -

clean:
	rm -f $(TTY_SERVER) $(TTY_SENDER)
	rm -fr frontend/out/
	@echo "Cleaned"

# Runs the server, without TLS/HTTPS (no need for localhost testing)
runs: $(TTY_SERVER)
	./$(TTY_SERVER) --url http://localhost:9090 --web_address :9090 --sender_address :7654 -frontend_path ./frontend/public

# Runs the sender, without TLS (no need for localhost testing)
runc: $(TTY_SENDER)
	./$(TTY_SENDER) --useTLS=false --server localhost:7654

test:
	@go test github.com/elisescu/tty-share/testing -v

FORCE:
