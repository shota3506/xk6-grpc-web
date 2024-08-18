package grpcweb

import (
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"google.golang.org/grpc/codes"
)

var _ modules.Module = (*RootModule)(nil)

type RootModule struct{}

func (m *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	metrics, err := registerMetrics(vu.InitEnv().Registry)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("failed to register gRPC Web module metrics: %w", err))
	}

	exports := make(map[string]any)
	exports["Client"] = func(_ sobek.ConstructorCall) *sobek.Object {
		rt := vu.Runtime()
		return rt.ToValue(newClient(vu, metrics)).ToObject(rt)
	}
	rt := vu.Runtime()
	exports["StatusOK"] = rt.ToValue(codes.OK)
	exports["StatusCanceled"] = rt.ToValue(codes.Canceled)
	exports["StatusUnknown"] = rt.ToValue(codes.Unknown)
	exports["StatusInvalidArgument"] = rt.ToValue(codes.InvalidArgument)
	exports["StatusDeadlineExceeded"] = rt.ToValue(codes.DeadlineExceeded)
	exports["StatusNotFound"] = rt.ToValue(codes.NotFound)
	exports["StatusAlreadyExists"] = rt.ToValue(codes.AlreadyExists)
	exports["StatusPermissionDenied"] = rt.ToValue(codes.PermissionDenied)
	exports["StatusResourceExhausted"] = rt.ToValue(codes.ResourceExhausted)
	exports["StatusFailedPrecondition"] = rt.ToValue(codes.FailedPrecondition)
	exports["StatusAborted"] = rt.ToValue(codes.Aborted)
	exports["StatusOutOfRange"] = rt.ToValue(codes.OutOfRange)
	exports["StatusUnimplemented"] = rt.ToValue(codes.Unimplemented)
	exports["StatusInternal"] = rt.ToValue(codes.Internal)
	exports["StatusUnavailable"] = rt.ToValue(codes.Unavailable)
	exports["StatusDataLoss"] = rt.ToValue(codes.DataLoss)
	exports["StatusUnauthenticated"] = rt.ToValue(codes.Unauthenticated)

	return &ModuleInstance{
		vu:      vu,
		exports: exports,
		metrics: metrics,
	}
}

var _ modules.Instance = (*ModuleInstance)(nil)

type ModuleInstance struct {
	vu modules.VU

	exports map[string]any
	metrics *instanceMetrics
}

func (i *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: i.exports,
	}
}
