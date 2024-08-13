package grpcweb

import "go.k6.io/k6/metrics"

type instanceMetrics struct{}

func registerMetrics(_ *metrics.Registry) (*instanceMetrics, error) {
	return &instanceMetrics{}, nil
}
