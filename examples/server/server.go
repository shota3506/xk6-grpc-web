package main

import (
	"context"
	"fmt"
	"time"

	helloworldpb "github.com/shota3506/xk6-grpc-web-example/helloworld"
	"google.golang.org/grpc"
)

type server struct {
	helloworldpb.UnimplementedGreeterServer
}

func (s *server) SayHello(_ context.Context, req *helloworldpb.HelloRequest) (*helloworldpb.HelloReply, error) {
	return &helloworldpb.HelloReply{
		Message: fmt.Sprintf("Hello! %s", req.GetName()),
	}, nil
}

func (s *server) SayRepeatHello(req *helloworldpb.RepeatHelloRequest, stream grpc.ServerStreamingServer[helloworldpb.HelloReply]) error {
	for i := range req.GetCount() {
		stream.Send(&helloworldpb.HelloReply{
			Message: fmt.Sprintf("Hello! %s%d", req.GetName(), i),
		})
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}
