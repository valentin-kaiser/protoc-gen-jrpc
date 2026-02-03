# protoc-gen-go-jrpc

A protoc compiler plugin for Go that generates server stubs and client implementations for JSON/RPC transport. It is inspired by gRPC but tailored for JSON/RPC.

## Overview

This plugin generates Go server stub interfaces and client implementations from protobuf service definitions that match specific method signatures expected by the go-core JSON/RPC API router.

> **Important**: This plugin only works in combination with the `protoc-gen-go` plugin and is meant to be used with the Go package `github.com/valentin-kaiser/go-core/web/jrpc`.

## Features

- ✅ **Server Stub Generation**: Generate server interfaces with proper method signatures
- ✅ **Client Implementation**: Generate typed HTTP/WebSocket clients for making RPC calls
- ✅ **Streaming Support**: Server streaming, client streaming, and bidirectional streaming
- ✅ **Protocol Buffer JSON**: Automatic marshaling/unmarshaling
- ✅ **Context Support**: Full context propagation for cancellation and timeouts
- ✅ **WebSocket Support**: Automatic WebSocket connections for all streaming patterns

## Installation

### Prerequisites

- [Protocol Buffers compiler (protoc)](https://protobuf.dev/installation/)
- [protoc-gen-go plugin](https://github.com/protocolbuffers/protobuf-go) 

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

func (s *userService) ListUsers(ctx context.Context, req *api.ListUsersRequest, out chan *api.ListUsersResponse) error {
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

### Server Methods

- **Unary**: `func(ctx context.Context, in *In) (*Out, error)`
- **Client stream**: `func(ctx context.Context, in chan *In) (*Out, error)`
- **Server stream**: `func(ctx context.Context, in *In, out chan *Out) error`
- **Bidi stream**: `func(ctx context.Context, in chan *In, out chan *Out) error`

### Client Methods

- **Unary**: `func(ctx context.Context, in *In) (*Out, error)` - HTTP POST
- **Server streaming**: `func(ctx context.Context, in *In, out chan *Out) error` - WebSocket
- **Client streaming**: `func(ctx context.Context, in chan *In) (*Out, error)` - WebSocket
- **Bidirectional streaming**: `func(ctx context.Context, in chan *In, out chan *Out) error` - WebSocket

## Client Usage

The plugin automatically generates client interfaces and implementations for your services. Clients automatically use HTTP for unary calls and WebSocket for streaming calls.

### Unary Call

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/valentin-kaiser/go-core/web/jrpc"
    "github.com/example/myapp/gen/go/api"
)

func main() {
    // Create a client with custom timeout
    client := api.NewUserServiceClient(
        "http://localhost:8080",
        jrpc.WithTimeout(10*time.Second),
    )
    
    // Make a unary call (uses HTTP)
    ctx := context.Background()
    resp, err := client.GetUser(ctx, &api.GetUserRequest{
        Id: "123",
    })
    if err != nil {
        log.Fatalf("Failed to get user: %v", err)
    }
    
    log.Printf("User: %s (%s)", resp.Name, resp.Email)
}
```

### Server Streaming

```go
func main() {
    client := api.NewUserServiceClient("http://localhost:8080")
    
    // Create output channel
    out := make(chan *api.ListUsersResponse, 10)
    
    // Consume stream
    go func() {
        for resp := range out {
            for _, user := range resp.Users {
                fmt.Printf("User: %s\n", user.Name)
            }
        }
    }()
    
    // Start streaming (uses WebSocket)
    ctx := context.Background()
    err := client.ListUsers(ctx, &api.ListUsersRequest{Limit: 10}, out)
    if err != nil {
        log.Fatalf("Stream failed: %v", err)
    }
}
```

See [EXAMPLE_CLIENT.md](EXAMPLE_CLIENT.md) for more detailed client examples.

## Generated Code

For each service defined in your `.proto` file, the plugin generates:

### Server Side
- `{Service}Server` interface with all RPC methods
- `Unimplemented{Service}Server` struct for forward compatibility
- `Register{Service}Server` function to register with the jRPC router

### Client Side
- `{Service}Client` interface with all unary RPC methods
- `{Service}ClientImpl` struct implementing the client interface
- `New{Service}Client` constructor function

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## License

This project is licensed under the BSD 3-Clause License - see the [LICENSE](LICENSE) file for details.
