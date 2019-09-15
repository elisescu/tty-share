DEPS=github.com/elisescu/pty github.com/sirupsen/logrus golang.org/x/crypto/ssh/terminal github.com/gorilla/mux github.com/gorilla/websocket github.com/go-bindata/go-bindata/...
DEST_DIR=./out
TTY_SERVER=$(DEST_DIR)/tty-server
TTY_SHARE=$(DEST_DIR)/tty-share

# We need to make sure the assets_bundle is in the list only onces in both these two special cases:
# a) first time, when the assets_bundle.go is generated, and b) when it's already existing there,
# but it has to be re-generated.
# Unfortunately, the assets_bundle.go seems to have to be in the same folder as the rest of the
# server sources, so that's why all this mess
TTY_SERVER_SRC=$(filter-out ./tty-server/assets_bundle.go, $(wildcard ./tty-server/*.go)) ./tty-server/assets_bundle.go
TTY_SHARE_SRC=$(wildcard ./tty-share/*.go)
COMMON_SRC=$(wildcard ./common/*go)
TTY_SERVER_ASSETS=$(wildcard frontend/public/*)


## Keep this as the first and default target, so no need to mess up with building the frontend&rest if the server side is not needed
$(TTY_SHARE): get-deps $(TTY_SHARE_SRC) $(COMMON_SRC)
	go build -o $@ $(TTY_SHARE_SRC)

## Build both the server and the tty-share
all: get-deps $(TTY_SHARE) $(TTY_SERVER)
	@echo "All done"

get-deps:
	go get $(DEPS)

# Building the server and tty-share
$(TTY_SERVER): get-deps $(TTY_SERVER_SRC) $(COMMON_SRC)
	go build -o $@ $(TTY_SERVER_SRC)

tty-server/assets_bundle.go: $(TTY_SERVER_ASSETS)
	go-bindata --prefix frontend/public/ -o $@ $^

%.zip: %
	zip $@ $^

frontend: force
	cd frontend && npm install && npm run build && cd -
force:

# Other different targets

## tty-share release binaries for Linux and OSX
# tty-share: $(OUT_DIR)/tty-share.osx $(OUT_DIR)/tty-share.linux
release: $(TTY_SHARE).osx.zip $(TTY_SHARE).lin.zip
	@echo "Done: " $@

$(TTY_SHARE).lin: $(TTY_SHARE_SRC) $(COMMON_SRC)
	GOOS=linux go build -o $@ $(TTY_SHARE_SRC)

$(TTY_SHARE).osx: $(TTY_SHARE_SRC) $(COMMON_SRC)
	GOOS=darwin go build -o $@ $(TTY_SHARE_SRC)

clean:
	rm -fr out/
	rm -fr frontend/public
	@echo "Cleaned"

## Development helper targets
### Runs the server, without TLS/HTTPS (no need for localhost testing)
runs: $(TTY_SERVER)
	$(TTY_SERVER) --url http://localhost:9090 --web_address :9090 --sender_address :7654 -frontend_path ./frontend/public
### Runs the sender, without TLS (no need for localhost testing)
runc: $(TTY_SHARE)
	$(TTY_SHARE) --useTLS=false --server localhost:7654

test:
	@go test github.com/elisescu/tty-share/testing -v
