#!/bin/bash -eu
#
# useage ./go.sh <version>
# ex)
# ./go.sh 1.20.0

GO_VERSION=$1

docker run --rm -it --name go${GO_VERSION} -v "$(pwd)":/app -w /app golang:${GO_VERSION}-bullseye
