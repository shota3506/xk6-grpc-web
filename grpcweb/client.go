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
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/grafana/sobek"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/types"
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

	fdset, err := c.reflectServer(ctx, c.addr, p.metadata)
	if err != nil {
		return false, err
	}
	_, err = c.registerMethods(fdset)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *client) reflectServer(ctx context.Context, addr *url.URL, header http.Header) (*descriptorpb.FileDescriptorSet, error) {
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

	opts := []grpcreflect.ClientStreamOption{}
	if header != nil {
		opts = append(opts, grpcreflect.WithRequestHeaders(header))
	}

	stream := client.NewStream(ctx, opts...)
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

	connectReq, ctm, timeout, err := c.buildRequest(md, req, params)
	if err != nil {
		return nil, err
	}
	c.setSystemTags(ctm, c.addr, method)

	if timeout <= 0 {
		// default timeout is 2 minutes
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(c.vu.Context(), timeout)
	defer cancel()

	resp, err := c.callUnary(ctx, method, connectReq, ctm)
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

func (c *client) AsyncInvoke(method string, req sobek.Value, params sobek.Value) *sobek.Promise {
	promise, resolve, reject := c.vu.Runtime().NewPromise()

	md, ok := c.mds[method]
	if !ok {
		reject(fmt.Errorf("method %s not found in file descriptors", method))
		return promise
	}
	if req == nil {
		reject(fmt.Errorf("request cannot be nil"))
		return promise
	}

	connectReq, ctm, timeout, err := c.buildRequest(md, req, params)
	if err != nil {
		reject(err)
		return promise
	}
	c.setSystemTags(ctm, c.addr, method)

	callback := c.vu.RegisterCallback()

	if timeout <= 0 {
		// default timeout is 2 minutes
		timeout = 2 * time.Minute
	}

	go func() {
		ctx, cancel := context.WithTimeout(c.vu.Context(), timeout)
		defer cancel()

		resp, err := c.callUnary(ctx, method, connectReq, ctm)

		callback(func() error {
			if err != nil {
				var connectErr *connect.Error
				if errors.As(err, &connectErr) {
					resolve(&invokeResponse{
						Error:        connectErr.Message(),
						ErrorDetails: connectErr.Details(),
						Status:       codes.Code(uint32(connectErr.Code())),
					})
					return nil
				}
				reject(err)
				return nil // do not return error
			}

			message, err := convertMessageToJSON(md, resp.Msg.data)
			if err != nil {
				reject(err)
				return nil // do not return error
			}

			resolve(&invokeResponse{
				Header:  resp.Header(),
				Trailer: resp.Trailer(),
				Message: message,
			})
			return nil
		})
	}()

	return promise
}

func (c *client) callUnary(ctx context.Context, method string, req *connect.Request[dynamicpb.Message], ctm *metrics.TagsAndMeta) (*connect.Response[deferredMessage], error) {
	client := connect.NewClient[dynamicpb.Message, deferredMessage](c.httpClient, c.addr.JoinPath(method).String(),
		connect.WithCodec(protoCodec{}),
		connect.WithGRPCWeb(),
	)

	beginTime := time.Now()
	resp, err := client.CallUnary(ctx, req)
	endTime := time.Now()

	// push metrics
	state := c.vu.State()
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: state.BuiltinMetrics.GRPCReqDuration,
			Tags:   ctm.Tags,
		},
		Time:     endTime,
		Metadata: ctm.Metadata,
		Value:    metrics.D(endTime.Sub(beginTime)),
	})

	return resp, err
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

	connectReq, ctm, timeout, err := c.buildRequest(md, req, params)
	if err != nil {
		return nil, err
	}
	c.setSystemTags(ctm, c.addr, method)

	ctx := c.vu.Context()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}

	s := &stream{
		vu:             c.vu,
		metrics:        c.metrics,
		tagsAndMeta:    ctm,
		client:         client,
		md:             md,
		eventListeners: newEventListeners(),
		tq:             taskqueue.New(c.vu.RegisterCallback),
		cancel:         cancel,
	}

	if err := s.begin(ctx, connectReq); err != nil {
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
	metadata http.Header
	reflect  bool
}

func (c *client) parseConnectParams(params sobek.Value) (connectParams, error) {
	result := connectParams{
		reflect: false,
	}

	if common.IsNullish(params) {
		return result, nil
	}

	rt := c.vu.Runtime()
	paramsObject := params.ToObject(rt)

	for _, k := range paramsObject.Keys() {
		v := paramsObject.Get(k)

		switch k {
		case "reflect":
			var ok bool
			result.reflect, ok = v.Export().(bool)
			if !ok {
				return result, errors.New("reflect value must be boolean")
			}
		case "metadata":
			if common.IsNullish(v) {
				break
			}

			metadata, ok := v.Export().(map[string]any)
			if !ok {
				return connectParams{}, fmt.Errorf("metadata must be an object with key-value pairs")
			}
			for hk, hv := range metadata {
				// TODO: support Binary-valued keys
				value, ok := hv.(string)
				if !ok {
					return connectParams{}, fmt.Errorf("%s value must be string", hk)
				}
				result.metadata[hk] = append(result.metadata[hk], value)
			}
		}
	}

	return result, nil
}

type callParams struct {
	metadata    http.Header
	tagsAndMeta metrics.TagsAndMeta
	timeout     time.Duration
}

func (c *client) parseCallParams(params sobek.Value) (callParams, error) {
	rt := c.vu.Runtime()

	result := callParams{
		metadata:    http.Header{},
		tagsAndMeta: c.vu.State().Tags.GetCurrentValues(),
		timeout:     0,
	}

	if params != nil {
		paramsObject := params.ToObject(rt)
		for _, k := range paramsObject.Keys() {
			v := paramsObject.Get(k)

			switch k {
			case "metadata":
				if common.IsNullish(v) {
					break
				}

				metadata, ok := v.Export().(map[string]any)
				if !ok {
					return callParams{}, fmt.Errorf("metadata must be an object with key-value pairs")
				}
				for hk, hv := range metadata {
					// TODO: support Binary-valued keys
					value, ok := hv.(string)
					if !ok {
						return callParams{}, fmt.Errorf("%s value must be string", hk)
					}
					result.metadata[hk] = append(result.metadata[hk], value)
				}
			case "tags":
				if err := common.ApplyCustomUserTags(rt, &result.tagsAndMeta, paramsObject.Get(k)); err != nil {
					return result, fmt.Errorf("metric tags: %w", err)
				}
			case "timeout":
				timeout, err := types.GetDurationValue(v.Export())
				if err != nil {
					return result, fmt.Errorf("invalid timeout value: %w", err)
				}
				result.timeout = timeout
			}
		}
	}
	return result, nil
}

func (c *client) buildRequest(md protoreflect.MethodDescriptor, req sobek.Value, params sobek.Value) (*connect.Request[dynamicpb.Message], *metrics.TagsAndMeta, time.Duration, error) {
	rt := c.vu.Runtime()

	b, err := req.ToObject(rt).MarshalJSON()
	if err != nil {
		return nil, nil, 0, err
	}
	reqdm := dynamicpb.NewMessage(md.Input())
	err = protojson.Unmarshal(b, reqdm)
	if err != nil {
		return nil, nil, 0, err
	}

	r := connect.NewRequest(reqdm)

	p, err := c.parseCallParams(params)
	if err != nil {
		return nil, nil, 0, err
	}

	// headers
	for k, v := range p.metadata {
		r.Header()[k] = v
	}

	return r, &p.tagsAndMeta, p.timeout, nil
}

func (c *client) setSystemTags(ctm *metrics.TagsAndMeta, addr *url.URL, method string) {
	state := c.vu.State()
	if state.Options.SystemTags.Has(metrics.TagURL) {
		ctm.SetSystemTagOrMeta(metrics.TagURL, addr.JoinPath(method).String())
	}

	parts := strings.Split(method[1:], "/")
	if len(parts) == 2 {
		ctm.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagService, parts[0])
		ctm.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagMethod, parts[1])
	}
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
