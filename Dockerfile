FROM golang:alpine as builder

COPY . /go/src/github.com/DanInci/raspberry-projector/
WORKDIR /go/src/github.com/DanInci/raspberry-projector/

RUN apk add --no-cache git

# GO BUILD FOR RASPBERRY
# RUN set -eux -o pipefail && \
#     go get -d ./... && \
#     CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -a -installsuffix cgo -o main .

RUN set -eux -o pipefail && \
    go get -d ./... && \
    go build -a -installsuffix cgo -o main .

RUN set -eux -o pipefail && \
    go get github.com/derekparker/delve/cmd/dlv

RUN apk del git

RUN mkdir /app
RUN mv main /app/

WORKDIR /app

ENTRYPOINT ["dlv", "exec", "/app/main", "--listen=0.0.0.0:2345", "--headless", "--api-version=2", "--log"]

# FOR WHEN MOVING TO RASPBERRY
# FROM resin/rpi-raspbian:latest

# COPY --from=builder /go/bin/dlv /usr/local/bin/
# COPY --from=builder /go/src/github.com/DanInci/raspberry-projector/main /app/
# WORKDIR /app

# ENTRYPOINT ["dlv", "debug", "main", "--listen=0.0.0.0:2345", "--headless", "--api-version=2", "--log"]