FROM golang:alpine as builder

COPY . /go/src/github.com/DanInci/raspi-projector-backend/
WORKDIR /go/src/github.com/DanInci/raspi-projector-backend/

RUN apk add --no-cache git

RUN set -eux -o pipefail && \
    go get -d ./... && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -o main .

RUN apk del git