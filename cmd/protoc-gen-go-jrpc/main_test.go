package main

import (
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRelative bool
		wantModule   string
		wantErr      string
	}{
		{name: "empty", input: ""},
		{name: "source relative", input: "paths=source_relative", wantRelative: true},
		{name: "module only", input: "module=github.com/example", wantModule: "github.com/example"},
		{name: "combined with spaces", input: " paths=source_relative , module=github.com/example ", wantRelative: true, wantModule: "github.com/example"},
		{name: "unknown option", input: "paths=source_relative,foo=bar", wantErr: "unknown parameter: foo=bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseOptions(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if opts.relative != tt.wantRelative {
				t.Fatalf("expected relative=%v, got %v", tt.wantRelative, opts.relative)
			}

			if opts.module != tt.wantModule {
				t.Fatalf("expected module=%q, got %q", tt.wantModule, opts.module)
			}
		})
	}
}

func TestGenerate_SourceRelativePathOption(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{buildMerpTypesProto()})
	req.Parameter = proto.String("paths=source_relative")

	resp := generate(req)
	if resp.GetError() != "" {
		t.Fatalf("generate returned error: %s", resp.GetError())
	}

	if len(resp.File) != 1 {
		t.Fatalf("expected exactly one generated file, got %d", len(resp.File))
	}

	if got, want := normalizePath(resp.File[0].GetName()), "portal/v1/portal_jrpc.pb.go"; got != want {
		t.Fatalf("expected generated filename %q, got %q", want, got)
	}
}

func TestGenerate_ModulePathOption(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{buildMerpTypesProto()})
	req.Parameter = proto.String("module=github.com/example")

	resp := generate(req)
	if resp.GetError() != "" {
		t.Fatalf("generate returned error: %s", resp.GetError())
	}

	if len(resp.File) != 1 {
		t.Fatalf("expected exactly one generated file, got %d", len(resp.File))
	}

	if got, want := normalizePath(resp.File[0].GetName()), "portal/v1/portal_jrpc.pb.go"; got != want {
		t.Fatalf("expected generated filename %q, got %q", want, got)
	}
}

func TestGenerate_UnknownOptionReturnsError(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{buildMerpTypesProto()})
	req.Parameter = proto.String("bad=true")

	resp := generate(req)
	if resp.GetError() == "" {
		t.Fatal("expected generate to return an error")
	}

	if !strings.Contains(resp.GetError(), "Failed to parse options") {
		t.Fatalf("expected parse-options error, got: %s", resp.GetError())
	}
}

func TestGenerate_NoServicesProducesNoFile(t *testing.T) {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"empty/v1/empty.proto"},
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("empty/v1/empty.proto"),
				Package: proto.String("empty.v1"),
				Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/empty/v1;v1")},
			},
		},
	}

	resp := generate(req)
	if resp.GetError() != "" {
		t.Fatalf("generate returned error: %s", resp.GetError())
	}

	if len(resp.File) != 0 {
		t.Fatalf("expected no generated files, got %d", len(resp.File))
	}
}

func TestGenerate_StreamingMethodShapes(t *testing.T) {
	req := buildStreamingRequest()
	content := mustGenerateSingleContent(t, req)

	checks := []string{
		"Unary(ctx context.Context, in *Req) (*Res, error)",
		"ClientStream(ctx context.Context, in <-chan *Req) (*Res, error)",
		"ServerStream(ctx context.Context, in *Req, out chan<- *Res) error",
		"Bidi(ctx context.Context, in <-chan *Req, out chan<- *Res) error",
		"func (c *StreamServiceClient) ClientStream(ctx context.Context, in <-chan *Req) (*Res, error)",
		"func (c *StreamServiceClient) ServerStream(ctx context.Context, in *Req, out chan<- *Res) error",
		"func (c *StreamServiceClient) Bidi(ctx context.Context, in <-chan *Req, out chan<- *Res) error",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Fatalf("expected generated code to contain %q, got:\n%s", check, content)
		}
	}
}

func TestGenerate_DescriptorUsesGoDescriptorIdent(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{buildMerpTypesProto()})
	content := mustGenerateSingleContent(t, req)

	if !strings.Contains(content, "return File_portal_v1_portal_proto") {
		t.Fatalf("expected descriptor method to return nested descriptor ident, got:\n%s", content)
	}
}

func TestGenerate_CrossPackageReturnTypeUsesQualifiedAlias(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{buildMerpTypesProto()})
	content := mustGenerateSingleContent(t, req)

	if !strings.Contains(content, `v1 "github.com/example/merp/v1"`) {
		t.Fatalf("expected generated import alias v1 for merp package, got:\n%s", content)
	}

	if !strings.Contains(content, "CreateTenant(ctx context.Context, in *CreateTenantRequest) (*v1.AcceptedRevision, error)") {
		t.Fatalf("expected generated signature to use *v1.AcceptedRevision, got:\n%s", content)
	}
}

func TestGenerate_AliasCollisionUsesDistinctAliases(t *testing.T) {
	req := buildRequestForPortal([]*descriptorpb.FileDescriptorProto{
		buildMerpTypesProto(),
		buildAcmeTypesProto(),
	})
	content := mustGenerateSingleContent(t, req)

	aliases := extractImportAliases(content)
	merpAlias, ok := aliases["github.com/example/merp/v1"]
	if !ok {
		t.Fatalf("expected import for github.com/example/merp/v1, got:\n%s", content)
	}

	acmeAlias, ok := aliases["github.com/example/acme/v1"]
	if !ok {
		t.Fatalf("expected import for github.com/example/acme/v1, got:\n%s", content)
	}

	if merpAlias == acmeAlias {
		t.Fatalf("expected distinct aliases for colliding package names, got %q", merpAlias)
	}

	if !strings.Contains(content, "*"+merpAlias+".AcceptedRevision") {
		t.Fatalf("expected merp type reference to use alias %q, got:\n%s", merpAlias, content)
	}

	if !strings.Contains(content, "*"+acmeAlias+".AcmeRevision") {
		t.Fatalf("expected acme type reference to use alias %q, got:\n%s", acmeAlias, content)
	}
}

func mustGenerateSingleContent(t *testing.T, req *pluginpb.CodeGeneratorRequest) string {
	t.Helper()

	resp := generate(req)
	if resp.GetError() != "" {
		t.Fatalf("generate returned error: %s", resp.GetError())
	}

	if len(resp.File) != 1 {
		t.Fatalf("expected exactly one generated file, got %d", len(resp.File))
	}

	return resp.File[0].GetContent()
}

func buildRequestForPortal(externals []*descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorRequest {
	portal := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("portal/v1/portal.proto"),
		Package: proto.String("portal.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/portal/v1;v1")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("CreateTenantRequest")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("PortalService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("CreateTenant"),
						InputType:  proto.String(".portal.v1.CreateTenantRequest"),
						OutputType: proto.String(".merp.v1.AcceptedRevision"),
					},
				},
			},
		},
		Dependency: []string{"merp/v1/types.proto"},
	}

	for _, ext := range externals {
		if ext.GetName() == "acme/v1/types.proto" {
			portal.Dependency = append(portal.Dependency, ext.GetName())
			portal.Service[0].Method = append(portal.Service[0].Method, &descriptorpb.MethodDescriptorProto{
				Name:       proto.String("CreateAcme"),
				InputType:  proto.String(".portal.v1.CreateTenantRequest"),
				OutputType: proto.String(".acme.v1.AcmeRevision"),
			})
		}
	}

	protoFiles := append([]*descriptorpb.FileDescriptorProto{}, externals...)
	protoFiles = append(protoFiles, portal)

	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"portal/v1/portal.proto"},
		ProtoFile:      protoFiles,
	}
}

func buildMerpTypesProto() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("merp/v1/types.proto"),
		Package: proto.String("merp.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/merp/v1;v1")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("AcceptedRevision")},
		},
	}
}

func buildAcmeTypesProto() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("acme/v1/types.proto"),
		Package: proto.String("acme.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/acme/v1;v1")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("AcmeRevision")},
		},
	}
}

func buildStreamingRequest() *pluginpb.CodeGeneratorRequest {
	stream := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("stream/v1/stream.proto"),
		Package: proto.String("stream.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/example/stream/v1;v1")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Req")},
			{Name: proto.String("Res")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("StreamService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("Unary"),
						InputType:  proto.String(".stream.v1.Req"),
						OutputType: proto.String(".stream.v1.Res"),
					},
					{
						Name:            proto.String("ClientStream"),
						InputType:       proto.String(".stream.v1.Req"),
						OutputType:      proto.String(".stream.v1.Res"),
						ClientStreaming: proto.Bool(true),
					},
					{
						Name:            proto.String("ServerStream"),
						InputType:       proto.String(".stream.v1.Req"),
						OutputType:      proto.String(".stream.v1.Res"),
						ServerStreaming: proto.Bool(true),
					},
					{
						Name:            proto.String("Bidi"),
						InputType:       proto.String(".stream.v1.Req"),
						OutputType:      proto.String(".stream.v1.Res"),
						ClientStreaming: proto.Bool(true),
						ServerStreaming: proto.Bool(true),
					},
				},
			},
		},
	}

	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"stream/v1/stream.proto"},
		ProtoFile:      []*descriptorpb.FileDescriptorProto{stream},
	}
}

func extractImportAliases(content string) map[string]string {
	aliases := map[string]string{}
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s+"([^"]+)"\s*$`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		aliases[m[2]] = m[1]
	}
	return aliases
}

func normalizePath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
