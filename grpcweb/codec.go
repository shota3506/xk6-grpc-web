package grpcweb

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type deferredMessage struct {
	data []byte
}

type protoCodec struct{}

func (p protoCodec) Name() string {
	return "proto"
}

func (p protoCodec) Marshal(a any) ([]byte, error) {
	protoMessage, ok := a.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("cannot marshal: %T does not implement proto.Message", a)
	}

	options := proto.MarshalOptions{
		Deterministic: true,
	}
	return options.Marshal(protoMessage)
}

func (p protoCodec) Unmarshal(bytes []byte, a any) error {
	if deferred, ok := a.(*deferredMessage); ok {
		// must make a copy since Connect framework will re-use the byte slice
		deferred.data = make([]byte, len(bytes))
		copy(deferred.data, bytes)
		return nil
	}
	protoMessage, ok := a.(proto.Message)
	if !ok {
		return fmt.Errorf("cannot unmarshal: %T does not implement proto.Message", a)
	}

	return proto.Unmarshal(bytes, protoMessage)
}
