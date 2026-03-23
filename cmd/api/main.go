package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	hellov1 "github.com/ideagate/aigateway-core/gen/hello/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// helloServer implements hellov1.HelloServiceServer.
type helloServer struct {
	hellov1.UnimplementedHelloServiceServer
}

// SayHello returns a greeting for the requested name.
func (s *helloServer) SayHello(_ context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	name := req.GetName()
	if name == "" {
		name = "World"
	}
	return &hellov1.SayHelloResponse{
		Message: fmt.Sprintf("Hello, %s!", name),
	}, nil
}

func main() {
	port := flag.Int("port", 50051, "gRPC server port")
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	hellov1.RegisterHelloServiceServer(srv, &helloServer{})

	// Register reflection so tools like grpcurl can inspect the server.
	reflection.Register(srv)

	log.Printf("gRPC server listening on :%d", *port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
