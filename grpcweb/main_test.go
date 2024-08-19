package grpcweb_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	weatherpb "github.com/shota3506/xk6-grpc-web/grpcweb/internal/grpc/weather"
	"github.com/shota3506/xk6-grpc-web/grpcweb/internal/grpc/weatherstub"
)

var (
	weatherServiceServer = &weatherstub.WeatherServiceServer{}
	address              string

	noopLogger = &logrus.Logger{
		Out:       io.Discard,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
		ExitFunc:  os.Exit,
	}
)

func TestMain(m *testing.M) {
	const port = 50051

	// start grpc server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	weatherpb.RegisterWeatherServiceServer(server, weatherServiceServer)
	reflection.Register(server)

	go func() {
		if err := server.Serve(lis); err != nil {
			if !errors.Is(err, grpc.ErrServerStopped) {
				log.Fatalf("failed to serve: %v", err)
			}
		}
	}()

	// start envoy
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to connect to docker: %v", err)
	}
	pool.MaxWait = 10 * time.Second

	pwd, _ := os.Getwd()
	runOptions := &dockertest.RunOptions{
		Repository:   "envoyproxy/envoy",
		Tag:          "v1.31.0",
		ExposedPorts: []string{"8080/tcp", "9901/tcp"},
		Mounts: []string{
			pwd + "/internal/envoy/envoy.yaml:/etc/envoy/envoy.yaml",
		},
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	resource, err := pool.RunWithOptions(runOptions,
		func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{
				Name: "no",
			}
		},
	)
	if err != nil {
		log.Fatalf("failed to start resource: %v", err)
	}

	if err := pool.Retry(func() error {
		resp, err := http.Get(fmt.Sprintf("http://%s", resource.GetHostPort("9901/tcp")))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code is not 200: %d", resp.StatusCode)
		}
		return nil
	}); err != nil {
		log.Fatalf("could not connect to envoy: %s", err)
	}

	address = resource.GetHostPort("8080/tcp")

	code := m.Run()

	// stop envoy
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}

	server.Stop()

	os.Exit(code)
}

func newRuntime(t *testing.T) (*modulestest.Runtime, error) {
	runtime := modulestest.NewRuntime(t)

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fs := fsext.NewOsFs()

	runtime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
	runtime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

	return runtime, nil
}

func moveToExecutionPhase(runtime *modulestest.Runtime) {
	registry := metrics.NewRegistry()
	runtime.MoveToVUContext(&lib.State{
		Samples:        make(chan metrics.SampleContainer, 1e4),
		Dialer:         &net.Dialer{},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         noopLogger,
	})
}

type callRecorder struct {
	sync.Mutex
	calls []string
}

func (r *callRecorder) call(text string) {
	r.Lock()
	defer r.Unlock()
	r.calls = append(r.calls, text)
}
