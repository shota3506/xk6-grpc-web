package grpcweb

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/sobek"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type methodInfo struct {
	Package         string
	Service         string
	FullMethod      string
	grpc.MethodInfo `json:"-" js:"-"`
}

type client struct {
	vu modules.VU

	// load
	mds map[string]protoreflect.MethodDescriptor

	// connect
	addr       string
	httpClient *http.Client
}

func newClient(vu modules.VU) *client {
	return &client{
		vu: vu,
	}
}

func (c *client) Load(importPaths []string, filenames ...string) ([]methodInfo, error) {
	if state := c.vu.State(); state != nil {
		return nil, errors.New("load must be called in the init context")
	}

	initEnv := c.vu.InitEnv()
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}

	if len(importPaths) == 0 {
		importPaths = append(importPaths, initEnv.CWD.Path)
	}

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: false,
		Accessor: protoparse.FileAccessor(func(filename string) (io.ReadCloser, error) {
			absFilePath := initEnv.GetAbsFilePath(filename)
			return initEnv.FileSystems["file"].Open(absFilePath)
		}),
	}

	fds, err := parser.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	fdset := &descriptorpb.FileDescriptorSet{}

	seen := make(map[string]struct{})
	for _, fd := range fds {
		fdset.File = append(fdset.File, walkFileDescriptors(seen, fd)...)
	}
	return c.convertToMethodInfo(fdset)
}

func (c *client) Connect(addr string) (bool, error) {
	if state := c.vu.State(); state == nil {
		return false, common.NewInitContextError("connecting to a gRPC Web server in the init context is not supported")
	}

	c.addr = addr
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext:  c.vu.State().Dialer.DialContext,
			Proxy:        http.ProxyFromEnvironment,
			MaxIdleConns: 1,
		},
	}

	return true, nil

	// TODO: use reflection protocol
}

type invokeResponse struct {
	Header  http.Header
	Trailer http.Header
	Message any

	Error        string
	ErrorDetails []*connect.ErrorDetail
	Status       codes.Code
}

func (c *client) Invoke(method string, req sobek.Value, params sobek.Value) (*invokeResponse, error) {
	md, ok := c.mds[method]
	if !ok {
		return nil, fmt.Errorf("method %s not found in file descriptors", method)
	}
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	url, err := url.JoinPath(c.addr, method)
	if err != nil {
		return nil, err
	}
	client := connect.NewClient[dynamicpb.Message, deferredMessage](c.httpClient, url,
		connect.WithCodec(protoCodec{}),
		connect.WithGRPCWeb(),
	)

	connectReq, err := c.buildRequest(md, req, params)
	if err != nil {
		return nil, err
	}

	ctx := c.vu.Context()

	beginTime := time.Now()
	resp, err := client.CallUnary(ctx, connectReq)
	endTime := time.Now()

	// push metrics
	state := c.vu.State()
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GRPCReqDuration,
		},
		Time:  endTime,
		Value: metrics.D(endTime.Sub(beginTime)),
	})

	if err != nil {
		var connectErr *connect.Error
		if errors.As(err, &connectErr) {
			return &invokeResponse{
				Error:        connectErr.Message(),
				ErrorDetails: connectErr.Details(),
				Status:       codes.Code(uint32(connectErr.Code())),
			}, nil
		}
		return nil, err
	}

	message, err := handleConnectResponse(md, resp.Msg.data)
	if err != nil {
		return nil, err
	}

	return &invokeResponse{
		Header:  resp.Header(),
		Trailer: resp.Trailer(),
		Message: message,
	}, nil
}

func (c *client) Stream(method string, req, params sobek.Value) (*sobek.Object, error) {
	md, ok := c.mds[method]
	if !ok {
		return nil, fmt.Errorf("method %s not found in file descriptors", method)
	}

	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	url, err := url.JoinPath(c.addr, method)
	if err != nil {
		return nil, err
	}
	client := connect.NewClient[dynamicpb.Message, deferredMessage](c.httpClient, url,
		connect.WithCodec(protoCodec{}),
		connect.WithGRPCWeb(),
	)

	connectReq, err := c.buildRequest(md, req, params)
	if err != nil {
		return nil, err
	}

	s := &stream{
		vu:             c.vu,
		client:         client,
		md:             md,
		eventListeners: newEventListeners(),
		tq:             taskqueue.New(c.vu.RegisterCallback),
	}

	if err := s.begin(connectReq); err != nil {
		return nil, err
	}

	rt := c.vu.Runtime()
	return rt.ToValue(s).ToObject(rt), nil
}

func (c *client) Close() error {
	// noop
	return nil
}

func (c *client) convertToMethodInfo(fdset *descriptorpb.FileDescriptorSet) ([]methodInfo, error) {
	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, err
	}
	var rtn []methodInfo
	if c.mds == nil {
		// This allows us to call load() multiple times, without overwriting the
		// previously loaded definitions.
		c.mds = make(map[string]protoreflect.MethodDescriptor)
	}
	appendMethodInfo := func(
		fd protoreflect.FileDescriptor,
		sd protoreflect.ServiceDescriptor,
		md protoreflect.MethodDescriptor,
	) {
		name := fmt.Sprintf("/%s/%s", sd.FullName(), md.Name())
		c.mds[name] = md
		rtn = append(rtn, methodInfo{
			MethodInfo: grpc.MethodInfo{
				Name:           string(md.Name()),
				IsClientStream: md.IsStreamingClient(),
				IsServerStream: md.IsStreamingServer(),
			},
			Package:    string(fd.Package()),
			Service:    string(sd.Name()),
			FullMethod: name,
		})
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sds := fd.Services()
		for i := 0; i < sds.Len(); i++ {
			sd := sds.Get(i)
			mds := sd.Methods()
			for j := 0; j < mds.Len(); j++ {
				md := mds.Get(j)
				appendMethodInfo(fd, sd, md)
			}
		}

		messages := fd.Messages()

		stack := make([]protoreflect.MessageDescriptor, 0, messages.Len())
		for i := 0; i < messages.Len(); i++ {
			stack = append(stack, messages.Get(i))
		}

		for len(stack) > 0 {
			message := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			_, errFind := protoregistry.GlobalTypes.FindMessageByName(message.FullName())
			if errors.Is(errFind, protoregistry.NotFound) {
				err = protoregistry.GlobalTypes.RegisterMessage(dynamicpb.NewMessageType(message))
				if err != nil {
					return false
				}
			}

			nested := message.Messages()
			for i := 0; i < nested.Len(); i++ {
				stack = append(stack, nested.Get(i))
			}
		}

		return true
	})
	if err != nil {
		return nil, err
	}
	return rtn, nil
}

func (c *client) buildRequest(md protoreflect.MethodDescriptor, req sobek.Value, params sobek.Value) (*connect.Request[dynamicpb.Message], error) {
	rt := c.vu.Runtime()

	b, err := req.ToObject(rt).MarshalJSON()
	if err != nil {
		return nil, err
	}
	reqdm := dynamicpb.NewMessage(md.Input())
	err = protojson.Unmarshal(b, reqdm)
	if err != nil {
		return nil, err
	}

	r := connect.NewRequest(reqdm)

	if params != nil {
		paramsObject := params.ToObject(rt)
		for _, k := range paramsObject.Keys() {
			switch k {
			case "metadata":
				val := paramsObject.Get(k)
				if common.IsNullish(val) {
					break
				}

				headers, ok := val.Export().(map[string]any)
				if !ok {
					return nil, fmt.Errorf("metadata must be an object with key-value pairs")
				}

				for hk, hv := range headers {
					// TODO: support Binary-valued keys
					value, ok := hv.(string)
					if !ok {
						return nil, fmt.Errorf("%s value must be string", hk)
					}
					r.Header()[hk] = append(r.Header()[hk], value)
				}
			}
		}
	}

	return r, nil
}

func walkFileDescriptors(seen map[string]struct{}, fd *desc.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	fds := []*descriptorpb.FileDescriptorProto{}

	if _, ok := seen[fd.GetName()]; ok {
		return fds
	}
	seen[fd.GetName()] = struct{}{}
	fds = append(fds, fd.AsFileDescriptorProto())

	for _, dep := range fd.GetDependencies() {
		deps := walkFileDescriptors(seen, dep)
		fds = append(fds, deps...)
	}

	return fds
}

func handleConnectResponse(md protoreflect.MethodDescriptor, data []byte) (map[string]any, error) {
	msg := dynamicpb.NewMessage(md.Output())

	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}

	result, err := dynamicMessageToMap(data, msg)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func dynamicMessageToMap(in []byte, msg *dynamicpb.Message) (map[string]any, error) {
	err := proto.Unmarshal(in, msg)
	if err != nil {
		return nil, err
	}

	content, err := innerDynamicMessageToMap(msg)
	if err != nil {
		return nil, err
	}

	if content, ok := content.(map[string]any); ok {
		return content, nil
	}

	return nil, fmt.Errorf("failed to convert dynamic message to map")
}

func innerDynamicMessageToMap(msg protoreflect.Message) (any, error) {
	mapData := make(map[string]any, msg.Descriptor().Fields().Len())

	for i := 0; i < msg.Descriptor().Fields().Len(); i++ {
		field := msg.Descriptor().Fields().Get(i)
		switch field.Cardinality() {
		case protoreflect.Repeated:
			list := msg.Get(field).List()
			values := make([]any, 0, list.Len())
			for j := 0; j < list.Len(); j++ {
				v, err := fieldToValue(field.Kind(), list.Get(j))
				if err != nil {
					return nil, err
				}
				values = append(values, v)
			}
			mapData[string(field.Name())] = values
		default:
			value, err := fieldToValue(field.Kind(), msg.Get(field))
			if err != nil {
				return nil, err
			}
			mapData[string(field.Name())] = value
		}
	}
	return mapData, nil
}

func fieldToValue(kind protoreflect.Kind, value protoreflect.Value) (any, error) {
	switch kind {
	case protoreflect.BoolKind:
		return value.Bool(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return value.Int(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return value.Uint(), nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return value.Float(), nil
	case protoreflect.StringKind:
		return value.String(), nil
	case protoreflect.BytesKind:
		return value.Bytes(), nil
	case protoreflect.EnumKind:
		return value.Enum(), nil
	case protoreflect.MessageKind:
		return innerDynamicMessageToMap(value.Message())
	default:
		return nil, fmt.Errorf("unsupported field type: %s", kind)
	}
}
