# single stage: build and run in the same go image
FROM golang:1.26-alpine
WORKDIR /src

# copy go.mod and go.sum first so the download layer gets cached
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /usr/local/bin/server ./cmd/server

# not running as root
RUN adduser -D -H -u 10001 app
USER app

EXPOSE 8080
ENTRYPOINT ["server"]