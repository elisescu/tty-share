set -ev
VERSION=$(git describe --tags `git rev-list --tags --max-count=1` | awk '{print substr($1,2); }')
OUTDIR=out

mkdir -p ${OUTDIR} && \
    GOOS=linux GOARCH=arm GOARM=6 go build -ldflags "-X main.version=${VERSION}" -o ${OUTDIR}/tty-share.rpi && \
    GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o ${OUTDIR}/tty-share.lin && \
    GOOS=darwin go build -ldflags "-X main.version=${VERSION}" -o ${OUTDIR}/tty-share.mac && \
    zip ${OUTDIR}/tty-share.rpi.zip ${OUTDIR}/tty-share.rpi && \
    zip ${OUTDIR}/tty-share.lin.zip ${OUTDIR}/tty-share.lin && \
    zip ${OUTDIR}/tty-share.mac.zip ${OUTDIR}/tty-share.mac
