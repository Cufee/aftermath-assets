# Build the application
FROM golang:1.22.3-bookworm as builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY ./ ./

# build a fully standalone binary with zero dependencies
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=0 GOOS=linux go build -o /bin/app .

# Run
FROM ghcr.io/sonroyaalmerol/steam-depot-downloader:latest

COPY --from=builder /bin/app /usr/bin/app
COPY --from=builder /workspace/filelist.txt /downloader/filelist.txt

ENV DECRYPT_DIR_PATH=/downloader/decrypted
ENV DOWNLOADER_FILE_LIST=/downloader/filelist.txt
ENV DOWNLOADER_CMD_PATH=DepotDownloader

RUN mkdir -p $DECRYPT_DIR_PATH

ENTRYPOINT [ "app" ]