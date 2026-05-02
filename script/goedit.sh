#!/bin/bash -eu

version=$(go version | awk '{print substr($3,3)}')

go mod edit -go=${version}
