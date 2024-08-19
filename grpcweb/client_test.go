package grpcweb_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"

	xk6grpcweb "github.com/shota3506/xk6-grpc-web/grpcweb"
	weatherpb "github.com/shota3506/xk6-grpc-web/grpcweb/internal/grpc/weather"
)

func TestClient(t *testing.T) {
	replacer := strings.NewReplacer(
		"GRPC_WEB_ADDR", "http://"+address,
	)

	for _, tt := range []struct {
		name     string
		setup    func(*testing.T)
		initCode string
		code     string
	}{
		{
			name: "invoke",
			setup: func(t *testing.T) {
				weatherServiceServer.SetWeather(t, func(ctx context.Context, req *weatherpb.LocationRequest) (*weatherpb.WeatherResponse, error) {
					return &weatherpb.WeatherResponse{}, nil
				})
			},
			initCode: `
let client = new grpcweb.Client();
client.load([], "./internal/grpc/weather/weather_service.proto");
`,
			code: `
client.connect("GRPC_WEB_ADDR");
var resp = client.invoke("/weather.WeatherService/GetWeather", {})
if (resp.status !== grpcweb.StatusOK) {
  throw new Error("unexpected response status: " + resp.status)
}
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := newRuntime(t)
			require.NoError(t, err)

			if tt.setup != nil {
				tt.setup(t)
			}

			m, ok := new(xk6grpcweb.RootModule).NewModuleInstance(runtime.VU).(*xk6grpcweb.ModuleInstance)
			require.True(t, ok)
			require.NoError(t, runtime.VU.Runtime().Set("grpcweb", m.Exports().Named))

			// init phase
			_, err = runtime.VU.Runtime().RunString(replacer.Replace(tt.initCode))
			require.NoError(t, err)

			registry := metrics.NewRegistry()
			runtime.MoveToVUContext(&lib.State{
				Samples:        make(chan metrics.SampleContainer, 1e4),
				Dialer:         &net.Dialer{},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
				Logger:         noopLogger,
			})

			// vu phase
			_, err = runtime.RunOnEventLoop(replacer.Replace(tt.code))
			require.NoError(t, err)
		})
	}
}
