package grpcweb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xk6grpcweb "github.com/shota3506/xk6-grpc-web/grpcweb"
	weatherpb "github.com/shota3506/xk6-grpc-web/grpcweb/internal/grpc/weather"
)

func TestClient(t *testing.T) {
	replacer := strings.NewReplacer(
		"GRPC_WEB_ADDR", "http://"+address,
	)

	for _, tt := range []struct {
		name          string
		setup         func(*testing.T)
		initCode      string
		code          string
		expectedCalls []string
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
var resp = client.invoke("/weather.WeatherService/GetWeather", {});
if (resp.status !== grpcweb.StatusOK) {
  throw new Error("unexpected response status: " + resp.status);
}
`,
		},
		{
			name: "async invoke",
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
var resp = client.asyncInvoke("/weather.WeatherService/GetWeather", {}).then(function(resp) {
  if (resp.status !== grpcweb.StatusOK) {
    throw new Error("unexpected response status: " + resp.status);
  }
}, (err) => {
  throw new Error("unexpected error: " + err);
});
`,
		},
		{
			name: "invoke with reflection",
			setup: func(t *testing.T) {
				weatherServiceServer.SetWeather(t, func(ctx context.Context, req *weatherpb.LocationRequest) (*weatherpb.WeatherResponse, error) {
					return &weatherpb.WeatherResponse{}, nil
				})
			},
			initCode: `
let client = new grpcweb.Client();
`,
			code: `
client.connect("GRPC_WEB_ADDR", {
  reflect: true,
});
var resp = client.invoke("/weather.WeatherService/GetWeather", {});
if (resp.status !== grpcweb.StatusOK) {
  throw new Error("unexpected response status: " + resp.status);
}
`,
		},
		{
			name: "server streaming",
			setup: func(t *testing.T) {
				weatherServiceServer.SetStreamWeather(t, func(req *weatherpb.LocationRequest, stream weatherpb.WeatherService_StreamWeatherServer) error {
					for range 3 {
						stream.Send(&weatherpb.WeatherResponse{})
					}
					return nil
				})
			},
			initCode: `
let client = new grpcweb.Client();
client.load([], "./internal/grpc/weather/weather_service.proto");
`,
			code: `
client.connect("GRPC_WEB_ADDR");
const stream = client.stream("/weather.WeatherService/StreamWeather", {});
stream.on("data", (data) => {
  call("data")
});
stream.on("error", (e) => {
  call("error: " + e)
});
stream.on("end", () => {
  call("end")
  client.close();
});
`,
			expectedCalls: []string{
				`data`,
				`data`,
				`data`,
				`end`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := newRuntime(t)
			require.NoError(t, err)

			m, ok := new(xk6grpcweb.RootModule).NewModuleInstance(runtime.VU).(*xk6grpcweb.ModuleInstance)
			require.True(t, ok)
			require.NoError(t, runtime.VU.Runtime().Set("grpcweb", m.Exports().Named))
			recorder := &callRecorder{}
			require.NoError(t, runtime.VU.Runtime().Set("call", recorder.call))

			if tt.setup != nil {
				tt.setup(t)
			}

			// init phase
			_, err = runtime.VU.Runtime().RunString(replacer.Replace(tt.initCode))
			require.NoError(t, err)

			moveToExecutionPhase(runtime)

			// vu phase
			_, err = runtime.RunOnEventLoop(replacer.Replace(tt.code))
			require.NoError(t, err)

			require.Equal(t, tt.expectedCalls, recorder.calls)
		})
	}
}
