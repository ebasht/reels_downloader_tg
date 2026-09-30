FROM golang:1.26.6 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM python:3.13-slim
# Instagram changes often and yt-dlp has to follow; bump deliberately.
ARG YT_DLP_VERSION=2026.8.19
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir "yt-dlp[default,curl-cffi]==${YT_DLP_VERSION}"
COPY --from=build /out/bot /usr/local/bin/bot
ENV HOME=/tmp
USER nobody
ENTRYPOINT ["/usr/local/bin/bot"]
