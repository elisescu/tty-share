TTY_SERVER_ASSETS=$(wildcard frontend/public/*) frontend/public/index.html

.PHONY: all frontend clean cleanfront rebuild

all: assets_bundle.go
	@echo "Done"

rebuild: clean all

assets_bundle.go: $(TTY_SERVER_ASSETS)
	go get github.com/go-bindata/go-bindata/...
	go-bindata --prefix frontend/public/ -pkg server -o $@ frontend/public/*

frontend: cleanfront frontend/public/index.html assets_bundle.go

frontend/public/index.html:
	cd frontend && npm install && npm run build && cd -

cleanfront:
	rm -fr frontend/public

clean: cleanfront
	rm -fr assets_bundle.go
	@echo "Cleaned"
