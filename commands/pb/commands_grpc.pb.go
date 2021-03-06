// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package commands

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// CommandsClient is the client API for Commands service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type CommandsClient interface {
	ParseAndExec(ctx context.Context, in *Message, opts ...grpc.CallOption) (Commands_ParseAndExecClient, error)
}

type commandsClient struct {
	cc grpc.ClientConnInterface
}

func NewCommandsClient(cc grpc.ClientConnInterface) CommandsClient {
	return &commandsClient{cc}
}

func (c *commandsClient) ParseAndExec(ctx context.Context, in *Message, opts ...grpc.CallOption) (Commands_ParseAndExecClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Commands_serviceDesc.Streams[0], "/commands.Commands/parseAndExec", opts...)
	if err != nil {
		return nil, err
	}
	x := &commandsParseAndExecClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Commands_ParseAndExecClient interface {
	Recv() (*ReturnMessage, error)
	grpc.ClientStream
}

type commandsParseAndExecClient struct {
	grpc.ClientStream
}

func (x *commandsParseAndExecClient) Recv() (*ReturnMessage, error) {
	m := new(ReturnMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// CommandsServer is the server API for Commands service.
// All implementations must embed UnimplementedCommandsServer
// for forward compatibility
type CommandsServer interface {
	ParseAndExec(*Message, Commands_ParseAndExecServer) error
	mustEmbedUnimplementedCommandsServer()
}

// UnimplementedCommandsServer must be embedded to have forward compatible implementations.
type UnimplementedCommandsServer struct {
}

func (UnimplementedCommandsServer) ParseAndExec(*Message, Commands_ParseAndExecServer) error {
	return status.Errorf(codes.Unimplemented, "method ParseAndExec not implemented")
}
func (UnimplementedCommandsServer) mustEmbedUnimplementedCommandsServer() {}

// UnsafeCommandsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CommandsServer will
// result in compilation errors.
type UnsafeCommandsServer interface {
	mustEmbedUnimplementedCommandsServer()
}

func RegisterCommandsServer(s grpc.ServiceRegistrar, srv CommandsServer) {
	s.RegisterService(&_Commands_serviceDesc, srv)
}

func _Commands_ParseAndExec_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Message)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(CommandsServer).ParseAndExec(m, &commandsParseAndExecServer{stream})
}

type Commands_ParseAndExecServer interface {
	Send(*ReturnMessage) error
	grpc.ServerStream
}

type commandsParseAndExecServer struct {
	grpc.ServerStream
}

func (x *commandsParseAndExecServer) Send(m *ReturnMessage) error {
	return x.ServerStream.SendMsg(m)
}

var _Commands_serviceDesc = grpc.ServiceDesc{
	ServiceName: "commands.Commands",
	HandlerType: (*CommandsServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "parseAndExec",
			Handler:       _Commands_ParseAndExec_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "commands.proto",
}
