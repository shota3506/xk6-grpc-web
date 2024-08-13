// Package grpc exist just to register the grpc extension
package grpc

import (
	"github.com/shota3506/xk6-grpc-web/grpcweb"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/grpc-web", new(grpcweb.RootModule))
}
