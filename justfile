set shell := ["/usr/bin/env", "bash", "-c"]

default: install

fmt:
    go fmt ./...

run:
    go run main.go

build:
    go build -o ops main.go

tidy:
    go mod tidy

add PACKAGE:
    go get -u {{PACKAGE}}

install:
    go install