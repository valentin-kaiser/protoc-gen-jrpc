.PHONY: build build-linux build-windows clean

# Build for current platform
build:
	go build -o protoc-gen-go-jrpc$(shell go env GOEXE) ./cmd/protoc-gen-go-jrpc

# Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o protoc-gen-go-jrpc ./cmd/protoc-gen-go-jrpc

# Build for Windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o protoc-gen-go-jrpc.exe ./cmd/protoc-gen-go-jrpc

# Build for multiple platforms
build-all: build-linux build-windows

# Clean build artifacts
clean:
	rm -f protoc-gen-go-jrpc protoc-gen-go-jrpc.exe

# Install to GOPATH/bin
install:
	go install ./cmd/protoc-gen-go-jrpc
