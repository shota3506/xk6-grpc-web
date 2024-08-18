package grpcweb

import "go.k6.io/k6/metrics"

const (
	gRPCStreamsName                 = "grpc_streams"
	gRPCStreamsMessagesReceivedName = "grpc_streams_msgs_received"
)

type instanceMetrics struct {
	streams                 *metrics.Metric
	streamsMessagesReceived *metrics.Metric
}

func registerMetrics(registry *metrics.Registry) (*instanceMetrics, error) {
	streams, err := registry.NewMetric(gRPCStreamsName, metrics.Counter)
	if err != nil {
		return nil, err
	}

	streamsMessagesReceived, err := registry.NewMetric(gRPCStreamsMessagesReceivedName, metrics.Counter)
	if err != nil {
		return nil, err
	}

	return &instanceMetrics{
		streams:                 streams,
		streamsMessagesReceived: streamsMessagesReceived,
	}, nil
}
