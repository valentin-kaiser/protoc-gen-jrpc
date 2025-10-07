# protoc-gen-go-jrpc

A protoc compiler plugin for Go that generates server stubs for JSON/RPC transport.

## Overview

This plugin generates Go server stub interfaces from protobuf service definitions that match specific method signatures expected by the go-core JSON/RPC API router.

**Important**: This plugin only works in combination with the `protoc-gen-go` plugin and is meant to be used with the Go package `github.com/valentin-kaiser/go-core/web/jrpc`.

## Installation

### Prerequisites

- [Protocol Buffers compiler (protoc)](https://protobuf.dev/installation/)
- [protoc-gen-go](https://github.com/protocolbuffers/protobuf-go) plugin
- 
### Install protoc-gen-go-jrpc

```bash
go install github.com/valentin-kaiser/protoc-gen-go-jrpc/cmd/protoc-gen-go-jrpc@latest
```

Make sure your `$GOPATH/bin` (or `$GOBIN`) is in your `$PATH` so protoc can find the plugin.

## Usage

### Basic Example

1. Define your service in a `.proto` file:

```protobuf
syntax = "proto3";

package api;
option go_package = "github.com/example/myapp/gen/go/api";

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc ListUsers(ListUsersRequest) returns (stream ListUsersResponse);
}

message GetUserRequest {
  string id = 1;
}

message GetUserResponse {
  string id = 1;
  string name = 2;
  string email = 3;
}

message ListUsersRequest {
  int32 limit = 1;
}

message ListUsersResponse {
  repeated User users = 1;
}

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}
```

2. Generate Go code using both plugins:

```bash
protoc -I . \
  --go_out=./gen/go \
  --go_opt=module=github.com/example/myapp \
  --go-jrpc_out=./gen/go \
  --go-jrpc_opt=module=github.com/example/myapp \
  api.proto
```

3. Implement the generated interface in your Go application:

```go
package main

import (
    "context"
    "log"
    
    "github.com/valentin-kaiser/go-core/web/jrpc"
    "github.com/example/myapp/gen/go/api"
)

// Embed the generated UnimplementedUserServiceServer
type userService struct {
    api.UnimplementedUserServiceServer
}

func (s *userService) GetUser(ctx context.Context, req *api.GetUserRequest) (*api.GetUserResponse, error) {
    return &api.GetUserResponse{
        Id:    req.Id,
        Name:  "John Doe",
        Email: "john@example.com",
    }, nil
}

func (s *userService) ListUsers(ctx context.Context, req *api.ListUsersRequest, out chan api.ListUsersResponse) error {
    defer close(out)
    
    // Send users to the output channel
    out <- api.ListUsersResponse{
        Users: []*api.User{
            {Id: "1", Name: "Alice", Email: "alice@example.com"},
            {Id: "2", Name: "Bob", Email: "bob@example.com"},
        },
    }
    
    return nil
}

func main() {
    service := &userService{}
    
    // Create JRPC server with the service implementation
    server := jrpc.New(service)
    
    // Start your server (example using go-core/web)
    log.Println("Server starting on :8080...")
    // Use your preferred HTTP server setup here
}
```

## Method Signatures

The plugin generates methods with the following signatures based on streaming types:

- **Unary**: `func(ctx context.Context, in *In) (*Out, error)`
- **Client stream**: `func(ctx context.Context, in chan *In) (*Out, error)`
- **Server stream**: `func(ctx context.Context, in *In, out chan Out) error`
- **Bidi stream**: `func(ctx context.Context, in chan *In, out chan Out) error`