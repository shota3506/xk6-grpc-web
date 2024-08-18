package weatherstub

import (
	"context"
	"testing"

	weatherpb "github.com/shota3506/xk6-grpc-web/grpcweb/internal/grpc/weather"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ weatherpb.WeatherServiceServer = (*WeatherServiceServer)(nil)

type WeatherServiceServer struct {
	weatherpb.UnimplementedWeatherServiceServer

	weatherFunc       func(ctx context.Context, req *weatherpb.LocationRequest) (*weatherpb.WeatherResponse, error)
	streamWeatherFunc func(req *weatherpb.LocationRequest, stream grpc.ServerStreamingServer[weatherpb.WeatherResponse]) error
}

func (s *WeatherServiceServer) GetWeather(ctx context.Context, req *weatherpb.LocationRequest) (*weatherpb.WeatherResponse, error) {
	if s.weatherFunc != nil {
		return s.weatherFunc(ctx, req)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetWeather not implemented")
}

func (s *WeatherServiceServer) StreamWeather(req *weatherpb.LocationRequest, stream grpc.ServerStreamingServer[weatherpb.WeatherResponse]) error {
	if s.streamWeatherFunc != nil {
		return s.streamWeatherFunc(req, stream)
	}
	return status.Errorf(codes.Unimplemented, "method StreamWeather not implemented")
}

func (s *WeatherServiceServer) SetWeather(t *testing.T, f func(ctx context.Context, req *weatherpb.LocationRequest) (*weatherpb.WeatherResponse, error)) {
	prev := s.weatherFunc
	t.Cleanup(func() {
		s.weatherFunc = prev
	})
	s.weatherFunc = f
}

func (s *WeatherServiceServer) SetStreamWeather(t *testing.T, f func(req *weatherpb.LocationRequest, stream grpc.ServerStreamingServer[weatherpb.WeatherResponse]) error) {
	prev := s.streamWeatherFunc
	t.Cleanup(func() {
		s.streamWeatherFunc = prev
	})
	s.streamWeatherFunc = f
}
