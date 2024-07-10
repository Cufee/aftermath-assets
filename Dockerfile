# Download required assets/binaries
FROM debian:stable-slim as downloader

WORKDIR /tmp

RUN apt-get update
RUN apt-get install unzip wget -y

RUN wget https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_2.6.0/DepotDownloader-linux-x64.zip -O downloader.zip
RUN unzip downloader.zip -d /downloader

# Build the application
FROM golang:1.22.3-bookworm as builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY ./ ./

# build a fully standalone binary with zero dependencies
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=0 GOOS=linux go build -o /bin/app .

# Run
FROM scratch

COPY --from=downloader /downloader /downloader
COPY --from=builder /bin/app /usr/bin/app

ENV DOWNLOADER_CMD_PATH=/downloader/DepotDownloader

ENTRYPOINT [ "app" ]