TTY_SERVER=tty_server
TTY_SENDER=tty_sender

# We need to make sure the assets_bundle is in the list only onces in both these two special cases:
# a) first time, when the assets_bundle.go is generated, and b) when it's already existing there,
# but it has to be re-generated.
# Unfortunately, the assets_bundle.go seems to have to be in the same folder as the rest of the
# server sources, so that's why all this mess
TTY_SERVER_SRC=$(filter-out ./tty-server/assets_bundle.go, $(wildcard ./tty-server/*.go)) ./tty-server/assets_bundle.go
TTY_SENDER_SRC=$(wildcard ./tty-sender/*.go)
EXTRA_BUILD_DEPS=$(wildcard ./common/*go)
TTY_SERVER_ASSETS=$(addprefix ./public/templates/,$(notdir $(wildcard ./frontend/templates/*))) public/bundle.js

all: $(TTY_SERVER) $(TTY_SENDER)
	@echo "All done"

$(TTY_SERVER): $(TTY_SERVER_SRC) $(EXTRA_BUILD_DEPS) $(TTY_SERVER_ASSETS)
	go build -o $@ $(TTY_SERVER_SRC)

$(TTY_SENDER): $(TTY_SENDER_SRC) $(EXTRA_BUILD_DEPS)
	go build -o $@ $(TTY_SENDER_SRC)

# TODO: perhaps replace all these paths with variables?
frontend/bundle.js: $(wildcard ./frontend/src/*)
	cd frontend && npm run build && cd -

public/bundle.js: frontend/bundle.js
	mkdir -p $(dir $@)
	cp $^ $@

public/templates/%: frontend/templates/%
	mkdir -p $(dir $@)
	cp $^ $@

tty-server/assets_bundle.go: $(TTY_SERVER_ASSETS)
	go-bindata --prefix public -o $@ $^

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
