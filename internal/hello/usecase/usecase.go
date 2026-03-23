package usecase

import (
	"context"
	"fmt"

	hellov1 "github.com/ideagate/aigateway-core/gen/hello/v1"
)

// Usecase defines the application layer contract behind the Hello gRPC service.
type Usecase interface {
	SayHello(context.Context, *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error)
}

// impl is the default implementation of Usecase.
type impl struct{}

// New returns the default Hello usecase implementation.
func New() Usecase {
	return impl{}
}

// SayHello returns a greeting for the requested name.
func (impl) SayHello(_ context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	name := req.GetName()
	if name == "" {
		name = "World"
	}
	return &hellov1.SayHelloResponse{
		Message: fmt.Sprintf("Hello, %s!", name),
	}, nil
}
