# docker run -d -p 2222:2222 -v "$(pwd)/configs:/vision3/configs" -v "$(pwd)/data:/vision3/data" -v "$(pwd)/menus:/vision3/menus" vision3

FROM golang:1.24-alpine AS builder

# Install build dependencies including libssh-dev for CGO
RUN apk add --no-cache git gcc musl-dev libssh-dev

WORKDIR /vision3

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO for libssh support (required for SSH server)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /vision3/ViSiON3 ./cmd/vision3

FROM alpine:latest

# Install runtime dependencies (libssh required for SSH server)
RUN apk --no-cache add openssh-keygen libssh ca-certificates

WORKDIR /vision3

COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod a+x /usr/local/bin/docker-entrypoint.sh

COPY --from=builder /vision3/ViSiON3 .

# Copy template configs for initialization
COPY templates/ ./templates/

VOLUME /vision3/configs
VOLUME /vision3/menus
VOLUME /vision3/data

EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

CMD ["./ViSiON3"]
