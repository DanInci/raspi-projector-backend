#!/usr/bin/env bash

IMAGE_NAME="projector-build-image"
CONTAINER_NAME="projector-build"
BINARY_NAME="projector"

function clean() {
    command docker rm -f ${CONTAINER_NAME}
}

set -eux -o pipefail
trap clean exit

command docker build -t ${IMAGE_NAME} .
command docker create --name ${CONTAINER_NAME} ${IMAGE_NAME}
command docker cp ${CONTAINER_NAME}:/go/src/github.com/DanInci/raspi-projector-backend/main ./${BINARY_NAME}

exit