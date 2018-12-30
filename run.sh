#!/usr/bin/env bash

# wrapper for docker-compose
# all arguments are passed to docker-compose up

# stop the services and clean up docker images
function clean() {
    set +e
    if [ -z "${prog_args}" ] || [ -n "${prog_args/*-d*/}" ]; then
        docker-compose down -v
    fi
    docker image prune -f
}

# set common compose args
function docker-compose() {
    if [ "${!compose_args*}" = "compose_args" ] && [ ${#compose_args[@]} -gt 0 ]; then
        command docker-compose "${compose_args[@]}" "${@}"
    else
        command docker-compose "${@}"
    fi
}

set -eux -o pipefail

prog_args="${*:-}"
trap clean exit

"${RUN:-true}" || exit 0

declare -a deps=(raspberry-projector)

test ${#deps[@]} -eq 0 || docker-compose pull "${deps[@]}"
docker-compose build --pull
if [ ${#} -gt 0 ]; then
    docker-compose up -V "${@}"
else
    docker-compose up -V
fi
