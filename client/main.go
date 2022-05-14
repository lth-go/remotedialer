package main

import (
	"context"
	"log"

	"github.com/rancher/remotedialer"
	"github.com/rancher/remotedialer/api/proxy"
	"github.com/rancher/remotedialer/dummy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial(":8123", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	stream, err := proxy.NewProxyServiceClient(conn).ProxyStream(context.Background())
	if err != nil {
		log.Fatalf("rpc error: %s", err)
	}

	remotedialer.ClientConnect(context.Background(), dummy.NewGrpcStreamTunnel(stream))
}
