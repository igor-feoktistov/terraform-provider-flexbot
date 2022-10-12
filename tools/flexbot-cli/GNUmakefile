OSFLAG=$(shell go env GOHOSTOS)

default: build

build:
	# Build flexbot CLI tool
	go build -o flexbot flexbot.go

clean:
	@rm -f ansible-roles/flexbot/bin/*

ansible:
	# Build darwin-amd64 binary for ansible role
	GOOS=darwin GOARCH=amd64 go build -o ansible-roles/flexbot/bin/flexbot.darwin
	# Build linux-amd64 binary for ansible role
	GOOS=linux GOARCH=amd64 go build -o ansible-roles/flexbot/bin/flexbot.linux

PHONY: build
