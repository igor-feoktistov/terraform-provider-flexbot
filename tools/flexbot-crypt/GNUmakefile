default: build

build:
	GOOS=darwin GOARCH=amd64 go build -o encrypt.darwin flexbot-crypt.go
	GOOS=linux GOARCH=amd64 go build -o encrypt.linux flexbot-crypt.go
	GOOS=darwin GOARCH=amd64 go build -o decrypt.darwin flexbot-decrypt.go
	GOOS=linux GOARCH=amd64 go build -o decrypt.linux flexbot-decrypt.go

PHONY: build
