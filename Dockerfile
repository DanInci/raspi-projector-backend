FROM golang:alpine as builder

COPY . /go/src/github.com/DanInci/raspberry-projector/
WORKDIR /go/src/github.com/DanInci/raspberry-projector/

RUN apk add --no-cache git

RUN set -eux -o pipefail && \
    go get -d ./... && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -o main .

RUN apk del git