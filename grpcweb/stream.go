package grpcweb

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
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
	listeners map[string]*eventListener
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		listeners: map[string]*eventListener{
			eventTypeData:  newEventListener(),
			eventTypeError: newEventListener(),
			eventTypeEnd:   newEventListener(),
		},
	}
}

func (els *eventListeners) add(eventType string, fn func(sobek.Value) (sobek.Value, error)) error {
	if _, ok := els.listeners[eventType]; !ok {
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
	els.listeners[eventType].add(fn)
	return nil
}

type eventListener struct {
	list []func(sobek.Value) (sobek.Value, error)
}

func newEventListener() *eventListener {
	return &eventListener{}
}

func (el *eventListener) add(fn func(sobek.Value) (sobek.Value, error)) {
	el.list = append(el.list, fn)
}

type stream struct {
	vu     modules.VU
	client *connect.Client[dynamicpb.Message, deferredMessage]

	md protoreflect.MethodDescriptor

	eventListeners *eventListeners

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

	// start goroutine to handle stream events
	go func() {
		defer s.close()

		// read data
		for s.stream.Receive() {
			msg := s.stream.Msg()

			message, err := handleConnectResponse(s.md, msg.data)
			if err != nil {
				s.vu.State().Logger.Errorf("failed to unmarshal message: %v", err)
				continue
			}

			if err := s.callback(message); err != nil {
				s.vu.State().Logger.Errorf("%v", err)
			}
		}

		if err := s.stream.Err(); err != nil {
			if err := s.errorCallback(err); err != nil {
				s.vu.State().Logger.Errorf("%v", err)
			}
		}
	}()

	return nil
}

func (s *stream) callback(message map[string]any) error {
	rt := s.vu.Runtime()
	listeners := s.eventListeners.listeners[eventTypeData]

	for _, listener := range listeners.list {
		if _, err := listener(rt.ToValue(message)); err != nil {
			return err
		}
	}
	return nil
}

type streamError struct {
	Error        string
	ErrorDetails []*connect.ErrorDetail
	Status       codes.Code
}

func (s *stream) errorCallback(err error) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}

	rt := s.vu.Runtime()
	listeners := s.eventListeners.listeners[eventTypeError]

	for _, listener := range listeners.list {
		if _, err := listener(rt.ToValue(&streamError{
			Error:        connectErr.Message(),
			ErrorDetails: connectErr.Details(),
			Status:       codes.Code(uint32(connectErr.Code())),
		})); err != nil {
			return err
		}
	}
	return nil
}

func (s *stream) close() error {
	rt := s.vu.Runtime()
	listeners := s.eventListeners.listeners[eventTypeEnd]

	for _, listener := range listeners.list {
		if _, err := listener(rt.ToValue(struct{}{})); err != nil {
			return err
		}
	}
	return nil
}
