package rpc_test

import (
	"context"
	"fmt"
	"testing"

	"runtime.link/rpc"
)

type DoSomething struct{}

func (d DoSomething) LRPC() string { return "rpc_test.DoSomething" }

func (d DoSomething) Call(ctx context.Context, svc Service, _ struct{}) (struct{}, error) {
	fmt.Println("Hello World")
	return struct{}{}, nil
}

type MyAPI struct {
	DoSomething func(context.Context) error
}

type Service struct {
	RPC rpc.Transport
}

func NewService(API Service) MyAPI {
	rpc.HandleCall(API.RPC, API, DoSomething.Call)
	return MyAPI{
		DoSomething: API.doSomething,
	}
}

func (s Service) doSomething(ctx context.Context) error {
	fmt.Println("Hello World")
	return nil
}

func TestRPC(t *testing.T) {
	type Request struct {
		OnComplete rpc.Void
	}

	var transport = rpc.New()

	NewService(Service{RPC: transport})

	API := func(ctx context.Context, req Request) error {
		_, err := req.OnComplete.Call(ctx, transport, struct{}{})
		return err
	}

	API(context.TODO(), Request{
		OnComplete: rpc.Call(DoSomething{}),
	})
}
