FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.Version=${VERSION}" \
      -o /out/burnmail .

FROM alpine:3.24 AS runner

LABEL org.opencontainers.image.title="burnmail" \
      org.opencontainers.image.source="https://github.com/fraluc06/burnmail"

# Non-root user; its home holds the account (~/.burnmail.json) and cache
# (~/.burnmail-cache.json), persisted via the compose volume.
RUN addgroup -S burnmail && adduser -S -G burnmail -h /home/burnmail burnmail

WORKDIR /home/burnmail
USER burnmail:burnmail

ENV TERM=xterm-256color \
    HOME=/home/burnmail

COPY --from=builder /out/burnmail /usr/local/bin/burnmail

ENTRYPOINT ["/usr/local/bin/burnmail"]
CMD ["--help"]
