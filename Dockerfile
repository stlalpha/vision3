# docker run -d -p 2222:2222 -v "$(pwd)/configs:/vision3/configs" -v "$(pwd)/data:/vision3/data" -v "$(pwd)/menus:/vision3/menus" vision3

FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /vision3

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /vision3/ViSiON3 ./cmd/vision3

FROM alpine:latest
RUN apk --no-cache add openssh-keygen

WORKDIR /vision3

COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod a+x /usr/local/bin/docker-entrypoint.sh

COPY --from=builder /vision3/ViSiON3 .

VOLUME /vision3/configs
VOLUME /vision3/menus
VOLUME /vision3/data

EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

CMD ["./ViSiON3"]
