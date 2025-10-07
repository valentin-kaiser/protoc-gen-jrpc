# protoc-gen-go-jrpc

A protoc compiler plugin for Go that generates server stubs for JSON/RPC transport.

## Overview

This plugin generates Go server stub interfaces from protobuf service definitions that match specific method signatures expected by the go-core JSON/RPC API router.

## Method Signatures

The plugin generates methods with the following signatures based on streaming types:

- **Unary**: `func(ctx context.Context, in *In) (*Out, error)`
- **Client stream**: `func(ctx context.Context, in chan *In) (*Out, error)`
- **Server stream**: `func(ctx context.Context, in *In, out chan Out) error`
- **Bidi stream**: `func(ctx context.Context, in chan *In, out chan Out) error`