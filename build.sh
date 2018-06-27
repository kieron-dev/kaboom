#!/usr/bin/env bash

set -e

CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

docker build -t cflondonservices/kaboom -f Dockerfile .

docker push cflondonservices/kaboom

