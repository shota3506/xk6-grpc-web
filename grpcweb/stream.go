package grpcweb

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	eventTypeData  = "data"
	eventTypeError = "error"
	eventTypeEnd   = "end"
)

type eventListeners struct {
	mu        sync.RWMutex
	listeners map[string][]func(sobek.Value) (sobek.Value, error)
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		listeners: map[string][]func(sobek.Value) (sobek.Value, error){
			eventTypeData:  {},
			eventTypeError: {},
			eventTypeEnd:   {},
		},
	}
}

func (els *eventListeners) add(eventType string, fn func(sobek.Value) (sobek.Value, error)) error {
	els.mu.Lock()
	defer els.mu.Unlock()

	listeners, ok := els.listeners[eventType]
	if !ok {
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
	els.listeners[eventType] = append(listeners, fn)
	return nil
}

func (els *eventListeners) all(eventType string) func(yield func(int, func(sobek.Value) (sobek.Value, error)) bool) {
	return func(yield func(int, func(sobek.Value) (sobek.Value, error)) bool) {
		els.mu.RLock()
		defer els.mu.RUnlock()

		for i, v := range els.listeners[eventType] {
			if !yield(i, v) {
				return
			}
		}
	}
}

type stream struct {
	vu          modules.VU
	metrics     *instanceMetrics
	tagsAndMeta *metrics.TagsAndMeta

	client         *connect.Client[dynamicpb.Message, deferredMessage]
	md             protoreflect.MethodDescriptor
	eventListeners *eventListeners
	tq             *taskqueue.TaskQueue

	stream *connect.ServerStreamForClient[deferredMessage]
}

func (s *stream) On(eventType string, handler func(sobek.Value) (sobek.Value, error)) {
	if handler == nil {
		common.Throw(s.vu.Runtime(), fmt.Errorf("handler for %s event isn't a callable function", eventType))
	}

	if err := s.eventListeners.add(eventType, handler); err != nil {
		s.vu.State().Logger.Warnf("can't register %s event handler: %v", eventType, err)
	}
}

func (s *stream) begin(req *connect.Request[dynamicpb.Message]) error {
	ctx := s.vu.Context()

	stream, err := s.client.CallServerStream(ctx, req)
	if err != nil {
		return err
	}
	s.stream = stream

	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: s.metrics.streams,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     time.Now(),
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})

	// start goroutine to handle stream events
	go func() {
		defer s.tq.Close()
		defer s.queueClose()

		// read data
		for s.stream.Receive() {
			msg := s.stream.Msg()

			message, err := convertMessageToJSON(s.md, msg.data)
			if err != nil {
				s.vu.State().Logger.Errorf("failed to unmarshal message: %v", err)
				continue
			}

			s.queueCallback(message)
		}

		if err := s.stream.Err(); err != nil {
			var connectErr *connect.Error
			if errors.As(err, &connectErr) {
				s.queueError(connectErr)
			} else {
				s.vu.State().Logger.Errorf("unexpected error from server: %v", err)
			}
		}

	}()

	return nil
}

func (s *stream) queueCallback(message any) {
	metrics.PushIfNotDone(s.vu.Context(), s.vu.State().Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: s.metrics.streamsMessagesReceived,
			Tags:   s.tagsAndMeta.Tags,
		},
		Time:     time.Now(),
		Metadata: s.tagsAndMeta.Metadata,
		Value:    1,
	})

	s.tq.Queue(func() (err error) {
		rt := s.vu.Runtime()
		s.eventListeners.all(eventTypeData)(func(i int, f func(sobek.Value) (sobek.Value, error)) bool {
			if _, err = f(rt.ToValue(message)); err != nil {
				// quit the loop and return the error
				return false
			}
			return true
		})
		return
	})
}

type streamError struct {
	Error        string
	ErrorDetails []*connect.ErrorDetail
	Status       codes.Code
}

func (s *stream) queueError(connectErr *connect.Error) {
	s.tq.Queue(func() (err error) {
		rt := s.vu.Runtime()
		s.eventListeners.all(eventTypeError)(func(_ int, f func(sobek.Value) (sobek.Value, error)) bool {
			if _, err = f(rt.ToValue(&streamError{
				Error:        connectErr.Message(),
				ErrorDetails: connectErr.Details(),
				Status:       codes.Code(uint32(connectErr.Code())),
			})); err != nil {
				// quit the loop and return the error
				return false
			}
			return true
		})
		return
	})
}

func (s *stream) queueClose() {
	s.tq.Queue(func() (err error) {
		rt := s.vu.Runtime()
		s.eventListeners.all(eventTypeEnd)(func(_ int, f func(sobek.Value) (sobek.Value, error)) bool {
			if _, err = f(rt.ToValue(struct{}{})); err != nil {
				// quit the loop and return the error
				return false
			}
			return true
		})
		return
	})
}
