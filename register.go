// Package grpcweb provides a k6 extension for gRPC-Web.
package grpcweb

import (
	"github.com/shota3506/xk6-grpc-web/grpcweb"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/grpc-web", new(grpcweb.RootModule))
}
