# Download depot downloader
FROM alpine:3.19 AS downloader

# Use "latest" or a specific version like "3.4.0"
ARG DEPOT_DOWNLOADER_VERSION="latest"
ARG DEPOT_DOWNLOADER_PLATFORM="linux-x64"

ENV DEPOT_DOWNLOADER_ARCHIVE="DepotDownloader-${DEPOT_DOWNLOADER_PLATFORM}.zip"

RUN apk add --no-cache wget unzip ca-certificates && \
    if [ "${DEPOT_DOWNLOADER_VERSION}" = "latest" ]; then \
        DOWNLOAD_URL="https://github.com/SteamRE/DepotDownloader/releases/latest/download/${DEPOT_DOWNLOADER_ARCHIVE}"; \
    else \
        DEPOT_DOWNLOADER_SLUG="DepotDownloader_${DEPOT_DOWNLOADER_VERSION}"; \
        DOWNLOAD_URL="https://github.com/SteamRE/DepotDownloader/releases/download/${DEPOT_DOWNLOADER_SLUG}/${DEPOT_DOWNLOADER_ARCHIVE}"; \
    fi && \
    wget "${DOWNLOAD_URL}" -O /tmp/depot.zip && \
    mkdir /out && \
    unzip /tmp/depot.zip -d /out && \
    rm /tmp/depot.zip

# Build the application
FROM golang:1.22.3-bookworm as builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY ./ ./

# build a fully standalone binary with zero dependencies
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=0 GOOS=linux go build -o /bin/app .

# Run
FROM debian:12-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends libssl3 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=downloader /out/DepotDownloader /usr/bin/downloader

COPY --from=builder /bin/app /usr/bin/app
COPY --from=builder /workspace/filelist.txt /downloader/filelist.txt

ENV DOWNLOADER_CMD_PATH=downloader

ENV DECRYPT_DIR_PATH=/downloader/decrypted
ENV DOWNLOADER_FILE_LIST=/downloader/filelist.txt

RUN mkdir -p $DECRYPT_DIR_PATH

ENTRYPOINT [ "app" ]