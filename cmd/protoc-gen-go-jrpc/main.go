package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/valentin-kaiser/go-core/flag"
	"github.com/valentin-kaiser/go-core/version"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type generator struct {
	file        *protogen.File
	genFile     *protogen.GeneratedFile
	opts        *options
	packageName string
	importPath  string
}

var (
	contextContextIdent = protogen.GoIdent{GoName: "Context", GoImportPath: "context"}
	errorsNewIdent      = protogen.GoIdent{GoName: "New", GoImportPath: "errors"}
	urlURLIdent         = protogen.GoIdent{GoName: "URL", GoImportPath: "net/url"}
	urlParseIdent       = protogen.GoIdent{GoName: "Parse", GoImportPath: "net/url"}

	protoreflectFileDescriptorIdent = protogen.GoIdent{GoName: "FileDescriptor", GoImportPath: "google.golang.org/protobuf/reflect/protoreflect"}

	jrpcServiceIdent             = protogen.GoIdent{GoName: "Service", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcRegisterIdent            = protogen.GoIdent{GoName: "Register", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcClientIdent              = protogen.GoIdent{GoName: "Client", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcClientOptionIdent        = protogen.GoIdent{GoName: "ClientOption", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcNewClientIdent           = protogen.GoIdent{GoName: "NewClient", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcClientStreamIdent        = protogen.GoIdent{GoName: "ClientStream", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcServerStreamIdent        = protogen.GoIdent{GoName: "ServerStream", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
	jrpcBidirectionalStreamIdent = protogen.GoIdent{GoName: "BidirectionalStream", GoImportPath: "github.com/valentin-kaiser/go-core/web/jrpc"}
)

type options struct {
	relative bool
	module   string
}

func main() {
	flag.Unregister("debug")
	flag.Unregister("path")
	flag.Init()

	if flag.Help {
		flag.PrintHelp()
		return
	}

	if flag.Version {
		fmt.Println(version.String())
		return
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}

	var req pluginpb.CodeGeneratorRequest
	err = proto.Unmarshal(input, &req)
	if err != nil {
		log.Fatalf("Failed to unmarshal request: %v", err)
	}

	resp := generate(&req)
	output, err := proto.Marshal(resp)
	if err != nil {
		log.Fatalf("Failed to marshal response: %v", err)
	}

	_, err = os.Stdout.Write(output)
	if err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}
}

func generate(req *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	opts, err := parseOptions(req.GetParameter())
	if err != nil {
		return &pluginpb.CodeGeneratorResponse{
			Error: proto.String(fmt.Sprintf("Failed to parse options: %v", err)),
		}
	}

	gen, err := protogen.Options{}.New(req)
	if err != nil {
		return &pluginpb.CodeGeneratorResponse{
			Error: proto.String(fmt.Sprintf("Failed to create protogen: %v", err)),
		}
	}

	var files []*pluginpb.CodeGeneratorResponse_File
	for _, file := range gen.Files {
		if !file.Generate {
			continue
		}

		generated, err := generateFile(gen, file, opts)
		if err != nil {
			return &pluginpb.CodeGeneratorResponse{
				Error: proto.String(fmt.Sprintf("Failed to generate file %s: %v", file.Desc.Path(), err)),
			}
		}

		if generated != nil {
			files = append(files, generated)
		}
	}

	return &pluginpb.CodeGeneratorResponse{
		File:              files,
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}
}

func parseOptions(parameter string) (*options, error) {
	opts := &options{}

	if parameter == "" {
		return opts, nil
	}

	for _, param := range strings.Split(parameter, ",") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}

		switch {
		case param == "paths=source_relative":
			opts.relative = true
		case strings.HasPrefix(param, "module="):
			opts.module = strings.TrimPrefix(param, "module=")
		default:
			return nil, fmt.Errorf("unknown parameter: %s", param)
		}
	}

	return opts, nil
}

func generateFile(plugin *protogen.Plugin, file *protogen.File, opts *options) (*pluginpb.CodeGeneratorResponse_File, error) {
	if len(file.Services) == 0 {
		return nil, nil
	}

	base := strings.TrimSuffix(file.Desc.Path(), ".proto")
	finalFilename := base + "_jrpc.pb.go"

	goPackageOpt := ""
	if file.Desc.Options() != nil {
		if fileOpts := file.Desc.Options().(*descriptorpb.FileOptions); fileOpts != nil {
			goPackageOpt = fileOpts.GetGoPackage()
		}
	}

	packageName := string(file.GoPackageName)
	importPath := string(file.GoImportPath)

	if goPackageOpt != "" {
		importPath = goPackageOpt
		packageName = filepath.Base(goPackageOpt)

		if strings.Contains(goPackageOpt, ";") {
			parts := strings.Split(goPackageOpt, ";")
			importPath = parts[0]
			packageName = parts[1]
		}
	}

	if !opts.relative {
		outputPath := importPath

		if opts.module != "" && strings.HasPrefix(outputPath, opts.module) && outputPath != opts.module {
			outputPath = strings.TrimPrefix(outputPath, opts.module+"/")
		}
		if opts.module != "" && outputPath == opts.module {
			outputPath = ""
		}

		if outputPath != "" {
			importDir := strings.ReplaceAll(outputPath, ".", "/")
			finalFilename = filepath.Join(importDir, filepath.Base(base)+"_jrpc.pb.go")
		}
	}

	generator := &generator{
		file:        file,
		genFile:     plugin.NewGeneratedFile(finalFilename, file.GoImportPath),
		opts:        opts,
		packageName: packageName,
		importPath:  importPath,
	}

	return generator.generate(finalFilename)
}

func (g *generator) generate(filename string) (*pluginpb.CodeGeneratorResponse_File, error) {
	g.genFile.P("// Code generated by protoc-gen-go-jrpc. DO NOT EDIT.")
	g.genFile.P("// versions:")
	g.genFile.P("// - protoc-gen-go-jrpc ", version.String())
	g.genFile.P("// source: ", g.file.Desc.Path())
	g.genFile.P()
	g.genFile.P("package ", g.packageName)
	g.genFile.P()

	for _, service := range g.file.Services {
		g.service(service)
	}

	content, err := g.genFile.Content()
	if err != nil {
		return nil, fmt.Errorf("failed to render generated code for %s: %w", filename, err)
	}

	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(filename),
		Content: proto.String(string(content)),
	}, nil
}

func (g *generator) qualifiedGoIdent(ident protogen.GoIdent) string {
	return g.genFile.QualifiedGoIdent(ident)
}

func (g *generator) goType(message *protogen.Message) string {
	return g.qualifiedGoIdent(message.GoIdent)
}

func (g *generator) goTypeRef(message *protogen.Message) string {
	return "*" + g.goType(message)
}

func (g *generator) service(service *protogen.Service) {
	serviceName := string(service.Desc.Name())

	// Generate the service interface
	g.generateInterface(serviceName, service)

	// Generate the unimplemented server struct
	g.genFile.P("type Unimplemented", serviceName, "Server struct{}")
	g.genFile.P()

	// Generate Descriptor method
	g.genFile.P("func (Unimplemented", serviceName, "Server) Descriptor() ", g.qualifiedGoIdent(protoreflectFileDescriptorIdent), " {")
	g.genFile.P("return ", g.file.GoDescriptorIdent.GoName)
	g.genFile.P("}")
	g.genFile.P()

	// Generate methods for each RPC
	for _, method := range service.Methods {
		g.method(serviceName, method)
	}

	// Generate registration function
	g.generateRegistrationFunction(serviceName)

	// Generate client interface and implementation
	g.generateClientInterface(serviceName, service)
	g.generateClientStruct(serviceName, service)
}

func (g *generator) method(serviceName string, method *protogen.Method) {
	methodName := method.GoName
	inputType := g.goTypeRef(method.Input)
	outputType := g.goTypeRef(method.Output)
	contextType := g.qualifiedGoIdent(contextContextIdent)

	isClientStreaming := method.Desc.IsStreamingClient()
	isServerStreaming := method.Desc.IsStreamingServer()

	// Determine method signature based on streaming
	var signature, body string

	switch {
	case !isClientStreaming && !isServerStreaming:
		// Unary
		signature = fmt.Sprintf("func (Unimplemented%sServer) %s(ctx %s, in %s) (%s, error)",
			serviceName, methodName, contextType, inputType, outputType)
		body = g.errorReturn(serviceName, methodName, true)
	case isClientStreaming && !isServerStreaming:
		// Client streaming
		signature = fmt.Sprintf("func (Unimplemented%sServer) %s(ctx %s, in <-chan %s) (%s, error)",
			serviceName, methodName, contextType, inputType, outputType)
		body = g.errorReturn(serviceName, methodName, true)
	case !isClientStreaming && isServerStreaming:
		// Server streaming
		signature = fmt.Sprintf("func (Unimplemented%sServer) %s(ctx %s, in %s, out chan<- %s) error",
			serviceName, methodName, contextType, inputType, outputType)
		body = g.errorReturn(serviceName, methodName, false)
	case isClientStreaming && isServerStreaming:
		// Bidirectional streaming
		signature = fmt.Sprintf("func (Unimplemented%sServer) %s(ctx %s, in <-chan %s, out chan<- %s) error",
			serviceName, methodName, contextType, inputType, outputType)
		body = g.errorReturn(serviceName, methodName, false)
	}

	g.genFile.P(signature, " {")
	g.genFile.P(body)
	g.genFile.P("}")
	g.genFile.P()
}

func (g *generator) errorReturn(serviceName, methodName string, needsNilReturn bool) string {
	errorMsg := fmt.Sprintf("method %s.%s not implemented", serviceName, methodName)
	errorCall := fmt.Sprintf("%s(\"%s\")", g.qualifiedGoIdent(errorsNewIdent), errorMsg)

	if needsNilReturn {
		return fmt.Sprintf("return nil, %s", errorCall)
	} else {
		return fmt.Sprintf("return %s", errorCall)
	}
}

func (g *generator) generateInterface(serviceName string, service *protogen.Service) {
	g.genFile.P("// ", serviceName, "Server is the server API for ", serviceName, " service.")
	g.genFile.P("type ", serviceName, "Server interface {")

	// Add Descriptor method to interface
	g.genFile.P("Descriptor() ", g.qualifiedGoIdent(protoreflectFileDescriptorIdent))

	// Generate interface methods
	for _, method := range service.Methods {
		g.generateInterfaceMethod(method)
	}

	g.genFile.P("}")
	g.genFile.P()
}

func (g *generator) generateInterfaceMethod(method *protogen.Method) {
	methodName := method.GoName
	inputType := g.goTypeRef(method.Input)
	outputType := g.goTypeRef(method.Output)
	contextType := g.qualifiedGoIdent(contextContextIdent)

	isClientStreaming := method.Desc.IsStreamingClient()
	isServerStreaming := method.Desc.IsStreamingServer()

	// Generate method signature based on streaming type
	var signature string

	switch {
	case !isClientStreaming && !isServerStreaming:
		// Unary
		signature = fmt.Sprintf("%s(ctx %s, in %s) (%s, error)",
			methodName, contextType, inputType, outputType)
	case isClientStreaming && !isServerStreaming:
		// Client streaming
		signature = fmt.Sprintf("%s(ctx %s, in <-chan %s) (%s, error)",
			methodName, contextType, inputType, outputType)
	case !isClientStreaming && isServerStreaming:
		// Server streaming
		signature = fmt.Sprintf("%s(ctx %s, in %s, out chan<- %s) error",
			methodName, contextType, inputType, outputType)
	case isClientStreaming && isServerStreaming:
		// Bidirectional streaming
		signature = fmt.Sprintf("%s(ctx %s, in <-chan %s, out chan<- %s) error",
			methodName, contextType, inputType, outputType)
	}

	g.genFile.P(signature)
}

func (g *generator) generateClientInterface(serviceName string, service *protogen.Service) {
	g.genFile.P("// ", serviceName, "ClientDefinition is the client API for ", serviceName, " service.")
	g.genFile.P("type ", serviceName, "ClientDefinition interface {")

	// Generate interface methods for all streaming types
	for _, method := range service.Methods {
		g.generateClientInterfaceMethod(method)
	}

	g.genFile.P("}")
	g.genFile.P()
}

func (g *generator) generateClientInterfaceMethod(method *protogen.Method) {
	methodName := method.GoName
	inputType := g.goTypeRef(method.Input)
	outputType := g.goTypeRef(method.Output)
	contextType := g.qualifiedGoIdent(contextContextIdent)

	isClientStreaming := method.Desc.IsStreamingClient()
	isServerStreaming := method.Desc.IsStreamingServer()

	// Generate method signature based on streaming type
	var signature string

	switch {
	case !isClientStreaming && !isServerStreaming:
		// Unary
		signature = fmt.Sprintf("%s(ctx %s, in %s) (%s, error)",
			methodName, contextType, inputType, outputType)
	case isClientStreaming && !isServerStreaming:
		// Client streaming
		signature = fmt.Sprintf("%s(ctx %s, in <-chan %s) (%s, error)",
			methodName, contextType, inputType, outputType)
	case !isClientStreaming && isServerStreaming:
		// Server streaming
		signature = fmt.Sprintf("%s(ctx %s, in %s, out chan<- %s) error",
			methodName, contextType, inputType, outputType)
	case isClientStreaming && isServerStreaming:
		// Bidirectional streaming
		signature = fmt.Sprintf("%s(ctx %s, in <-chan %s, out chan<- %s) error",
			methodName, contextType, inputType, outputType)
	}

	g.genFile.P(signature)
}

func (g *generator) generateClientStruct(serviceName string, service *protogen.Service) {
	urlType := g.qualifiedGoIdent(urlURLIdent)
	urlParse := g.qualifiedGoIdent(urlParseIdent)
	jrpcClient := g.qualifiedGoIdent(jrpcClientIdent)
	jrpcClientOption := g.qualifiedGoIdent(jrpcClientOptionIdent)
	jrpcNewClient := g.qualifiedGoIdent(jrpcNewClientIdent)

	// Generate client struct
	g.genFile.P("type ", serviceName, "Client struct {")
	g.genFile.P("client *", jrpcClient)
	g.genFile.P("baseURL *", urlType)
	g.genFile.P("}")
	g.genFile.P()

	// Generate New client function
	g.genFile.P("// New", serviceName, "Client creates a new client for the ", serviceName, " service.")
	g.genFile.P("// It parses and validates the baseURL, returning an error if the URL is malformed.")
	g.genFile.P("func New", serviceName, "Client(baseURL string, opts ...", jrpcClientOption, ") (*", serviceName, "Client, error) {")
	g.genFile.P("parsedURL, err := ", urlParse, "(baseURL)")
	g.genFile.P("if err != nil {")
	g.genFile.P("return nil, err")
	g.genFile.P("}")
	g.genFile.P("return &", serviceName, "Client{")
	g.genFile.P("client: ", jrpcNewClient, "(opts...),")
	g.genFile.P("baseURL: parsedURL,")
	g.genFile.P("}, nil")
	g.genFile.P("}")
	g.genFile.P()

	// Generate client methods for all streaming types
	for _, method := range service.Methods {
		g.generateClientMethod(serviceName, method)
	}

	// Add compile-time interface check
	g.genFile.P("// Ensure ", serviceName, "Client implements ", serviceName, "ClientDefinition")
	g.genFile.P("var _ ", serviceName, "ClientDefinition = (*", serviceName, "Client)(nil)")
	g.genFile.P()
}

func (g *generator) generateClientMethod(serviceName string, method *protogen.Method) {
	methodName := method.GoName
	inputType := g.goType(method.Input)
	outputType := g.goType(method.Output)
	contextType := g.qualifiedGoIdent(contextContextIdent)
	jrpcClientStream := g.qualifiedGoIdent(jrpcClientStreamIdent)
	jrpcServerStream := g.qualifiedGoIdent(jrpcServerStreamIdent)
	jrpcBidirectionalStream := g.qualifiedGoIdent(jrpcBidirectionalStreamIdent)

	isClientStreaming := method.Desc.IsStreamingClient()
	isServerStreaming := method.Desc.IsStreamingServer()

	switch {
	case !isClientStreaming && !isServerStreaming:
		// Unary
		g.genFile.P(fmt.Sprintf("func (c *%sClient) %s(ctx %s, in *%s) (*%s, error) {",
			serviceName, methodName, contextType, inputType, outputType))
		g.genFile.P(fmt.Sprintf("u := c.baseURL.JoinPath(%q, %q)", serviceName, methodName))
		g.genFile.P(fmt.Sprintf("out := &%s{}", outputType))
		g.genFile.P("err := c.client.Call(ctx, u, in, out, nil)")
		g.genFile.P("if err != nil {")
		g.genFile.P("return nil, err")
		g.genFile.P("}")
		g.genFile.P("return out, nil")
		g.genFile.P("}")
		g.genFile.P()

	case isClientStreaming && !isServerStreaming:
		// Client streaming
		g.genFile.P(fmt.Sprintf("func (c *%sClient) %s(ctx %s, in <-chan *%s) (*%s, error) {",
			serviceName, methodName, contextType, inputType, outputType))
		g.genFile.P(fmt.Sprintf("u := c.baseURL.JoinPath(%q, %q)", serviceName, methodName))
		g.genFile.P(fmt.Sprintf("out := &%s{}", outputType))
		g.genFile.P("err := ", jrpcClientStream, "(c.client, ctx, u, in, out)")
		g.genFile.P("if err != nil {")
		g.genFile.P("return nil, err")
		g.genFile.P("}")
		g.genFile.P("return out, nil")
		g.genFile.P("}")
		g.genFile.P()

	case !isClientStreaming && isServerStreaming:
		// Server streaming
		g.genFile.P(fmt.Sprintf("func (c *%sClient) %s(ctx %s, in *%s, out chan<- *%s) error {",
			serviceName, methodName, contextType, inputType, outputType))
		g.genFile.P(fmt.Sprintf("u := c.baseURL.JoinPath(%q, %q)", serviceName, methodName))
		g.genFile.P(fmt.Sprintf("factory := func() *%s { return &%s{} }", outputType, outputType))
		g.genFile.P("return ", jrpcServerStream, "(c.client, ctx, u, in, out, factory)")
		g.genFile.P("}")
		g.genFile.P()

	case isClientStreaming && isServerStreaming:
		// Bidirectional streaming
		g.genFile.P(fmt.Sprintf("func (c *%sClient) %s(ctx %s, in <-chan *%s, out chan<- *%s) error {",
			serviceName, methodName, contextType, inputType, outputType))
		g.genFile.P(fmt.Sprintf("u := c.baseURL.JoinPath(%q, %q)", serviceName, methodName))
		g.genFile.P(fmt.Sprintf("factory := func() *%s { return &%s{} }", outputType, outputType))
		g.genFile.P("return ", jrpcBidirectionalStream, "(c.client, ctx, u, in, out, factory)")
		g.genFile.P("}")
		g.genFile.P()
	}

}

func (g *generator) generateRegistrationFunction(serviceName string) {
	g.genFile.P("// Register", serviceName, "Server registers a ", serviceName, "Server with the JSON-RPC service registry.")
	g.genFile.P("// It returns a *jrpc.Service that can be used to handle JSON-RPC requests.")
	g.genFile.P("func Register", serviceName, "Server(server ", serviceName, "Server) *", g.qualifiedGoIdent(jrpcServiceIdent), " {")
	g.genFile.P("return ", g.qualifiedGoIdent(jrpcRegisterIdent), "(server)")
	g.genFile.P("}")
	g.genFile.P()

	// Add compile-time interface check
	g.genFile.P("// Ensure Unimplemented", serviceName, "Server implements ", serviceName, "Server")
	g.genFile.P("var _ ", serviceName, "Server = (*Unimplemented", serviceName, "Server)(nil)")
	g.genFile.P()
}
