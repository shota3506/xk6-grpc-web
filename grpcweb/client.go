package grpcweb

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/grafana/sobek"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	vu      modules.VU
	metrics *instanceMetrics

	// load
	mds map[string]protoreflect.MethodDescriptor

	// connect
	addr       *url.URL
	httpClient *http.Client
}

func newClient(vu modules.VU, metrics *instanceMetrics) *client {
	return &client{
		vu:      vu,
		metrics: metrics,
		mds:     make(map[string]protoreflect.MethodDescriptor),
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
	return c.registerMethods(fdset)
}

func (c *client) Connect(addr string, params sobek.Value) (bool, error) {
	ctx := c.vu.Context()

	if state := c.vu.State(); state == nil {
		return false, common.NewInitContextError("connecting to a gRPC Web server in the init context is not supported")
	}

	p, err := c.parseConnectParams(params)
	if err != nil {
		return false, err
	}

	c.addr, err = url.Parse(addr)
	if err != nil {
		return false, err
	}
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext:       c.vu.State().Dialer.DialContext,
			Proxy:             http.ProxyFromEnvironment,
			MaxIdleConns:      1,
			ForceAttemptHTTP2: false,
		},
	}

	if !p.reflect {
		return true, nil
	}

	fdset, err := c.reflectServer(ctx, c.addr)
	if err != nil {
		return false, err
	}
	_, err = c.registerMethods(fdset)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *client) reflectServer(ctx context.Context, addr *url.URL) (*descriptorpb.FileDescriptorSet, error) {
	// use HTTP2 transport because gRPC server reflection service provides bidirectional streaming RPC
	transport := &http2.Transport{}
	if addr.Scheme != "https" {
		var dialer net.Dialer
		transport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		}
	}

	client := grpcreflect.NewClient(&http.Client{Transport: transport}, addr.String(),
		connect.WithGRPCWeb(),
	)

	stream := client.NewStream(ctx)
	defer stream.Close()

	names, err := stream.ListServices()
	if err != nil {
		return nil, err
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	for _, name := range names {
		fds, err := stream.FileContainingSymbol(name)
		if err != nil {
			return nil, err
		}
		fdset.File = append(fdset.File, fds...)
	}
	return fdset, nil
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

	client := connect.NewClient[dynamicpb.Message, deferredMessage](c.httpClient, c.addr.JoinPath(method).String(),
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

	message, err := convertMessageToJSON(md, resp.Msg.data)
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

	client := connect.NewClient[dynamicpb.Message, deferredMessage](c.httpClient, c.addr.JoinPath(method).String(),
		connect.WithCodec(protoCodec{}),
		connect.WithGRPCWeb(),
	)

	connectReq, err := c.buildRequest(md, req, params)
	if err != nil {
		return nil, err
	}

	s := &stream{
		vu:             c.vu,
		metrics:        c.metrics,
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

func (c *client) registerMethods(fdset *descriptorpb.FileDescriptorSet) ([]methodInfo, error) {
	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, err
	}

	var info []methodInfo
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sds := fd.Services()
		for i := 0; i < sds.Len(); i++ {
			sd := sds.Get(i)
			mds := sd.Methods()
			for j := 0; j < mds.Len(); j++ {
				md := mds.Get(j)

				name := fmt.Sprintf("/%s/%s", sd.FullName(), md.Name())
				c.mds[name] = md
				info = append(info, methodInfo{
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
		}
		return true
	})
	return info, nil
}

type connectParams struct {
	reflect bool
}

func (c *client) parseConnectParams(params sobek.Value) (connectParams, error) {
	result := connectParams{
		reflect: false,
	}

	if common.IsNullish(params) {
		return result, nil
	}

	rt := c.vu.Runtime()
	object := params.ToObject(rt)

	for _, k := range object.Keys() {
		v := object.Get(k).Export()

		switch k {
		case "reflect":
			var ok bool
			result.reflect, ok = v.(bool)
			if !ok {
				return result, errors.New("reflect value must be boolean")
			}
		}
	}

	return result, nil
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
			v := paramsObject.Get(k)

			switch k {
			case "metadata":
				if common.IsNullish(v) {
					break
				}

				headers, ok := v.Export().(map[string]any)
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

func convertMessageToJSON(md protoreflect.MethodDescriptor, data []byte) (any, error) {
	msg := dynamicpb.NewMessage(md.Output())
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the message: %w", err)
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}
	raw, err := marshaler.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the message into JSON: %w", err)
	}

	var resp any
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the JSON message: %w", err)
	}
	return resp, nil
}
