package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	helloworldpb "github.com/shota3506/xk6-grpc-web-example/helloworld"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port    = flag.Int("port", 50051, "The server port")
	reflect = flag.Bool("reflect", false, "Enable server reflection service")
)

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	helloworldpb.RegisterGreeterServer(s, &server{})
	if *reflect {
		reflection.Register(s)
	}

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
