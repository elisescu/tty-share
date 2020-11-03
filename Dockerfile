FROM alpine:3.12

ARG build_deps="go git"

COPY . /go/src/github.com/elisescu/tty-share

RUN apk update && apk add -u $build_deps


RUN cd /go/src/github.com/elisescu/tty-share && \
    GOPATH=/go go get github.com/go-bindata/go-bindata/... && \
    GOPATH=/go /go/bin/go-bindata --prefix server/frontend/static -o gobindata.go server/frontend/static/* && \
    GOPATH=/go go build && \
    cp tty-share /usr/bin/ && \
    rm -r /go && \
    apk del $build_deps

ENTRYPOINT ["/usr/bin/tty-share", "--command", "/bin/sh"]
