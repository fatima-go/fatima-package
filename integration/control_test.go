package integration

import (
	"context"
	"fmt"
	"github.com/fatima-go/fatima-cmd/cipher"
	"github.com/fatima-go/fatima-cmd/config"
	"github.com/fatima-go/fatima-cmd/controlui"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/operations"
	"github.com/fatima-go/fatima-core/opm/transport"
	control "github.com/fatima-go/juno/control"
	jupiter "github.com/fatima-go/jupiter/deployment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync/atomic"
	"testing"
	"time"
)

type controlLab struct {
	cfg      config.JupiterContextRecord
	calls    atomic.Int32
	pid      atomic.Int32
	server   *control.Server
	gateway  *jupiter.Server
	endpoint string
}

func newControlLab(t *testing.T) *controlLab {
	t.Helper()
	l := &controlLab{}
	l.pid.Store(123)
	var target *api.Target
	j, err := jupiter.New(t.TempDir(), func(user, password string) (string, error) {
		if user == "operator" {
			return "OPERATOR", nil
		}
		if user == "monitor" {
			return "MONITOR", nil
		}
		return "", fmt.Errorf("bad user")
	}, func() ([]*api.Target, error) { return []*api.Target{target}, nil })
	if err != nil {
		t.Fatal(err)
	}
	l.gateway = j
	g := grpc.NewServer()
	j.Register(g)
	endpoint := serve(t, g, j.Capabilities())
	c, _ := transport.Dial(endpoint)
	s, err := control.New(t.TempDir(), "host:default", func(ctx context.Context, role string) error {
		_, e := api.NewIdentityClient(c).Validate(transport.WithToken(ctx, transport.Token(ctx)), &api.ValidateRequest{Role: role})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	l.server = s
	s.ListJobs = func() (*api.CronCatalog, error) {
		return &api.CronCatalog{PackageId: "host:default", Jobs: []*api.CronEntry{{Process: "sample", Name: "run", Spec: "@hourly"}}}, nil
	}
	s.RunJob = func(context.Context, *api.CronRequest) (string, error) {
		l.calls.Add(1)
		return "request delivered; result unknown", nil
	}
	s.Processes = func(*api.ProcessQuery) (*api.ProcessCatalog, error) {
		pid := l.pid.Load()
		state := "ALIVE"
		if pid == 0 {
			state = "DEAD"
		}
		return &api.ProcessCatalog{PackageId: "host:default", ObservedAt: time.Now().Unix(), Processes: []*api.ProcessEntry{{Name: "sample", State: state, Pid: fmt.Sprint(pid), Group: "SVC"}}}, nil
	}
	s.StopProcesses = func(ctx context.Context, names []string, emit operations.Emit) error {
		l.calls.Add(1)
		if names[0] == "fail" {
			return fmt.Errorf("injected stop failure")
		}
		for _, stage := range []string{"goaway", "shutdown"} {
			if err := emit("sample/"+stage, "WAITING", stage+" waiting", 0, 1); err != nil {
				return err
			}
			time.Sleep(150 * time.Millisecond)
			if err := emit("sample/"+stage, "SUCCEEDED", stage+" finished", 1, 1); err != nil {
				return err
			}
		}
		return nil
	}
	s.StartProcesses = func(ctx context.Context, names []string, emit operations.Emit) error {
		l.calls.Add(1)
		if names[0] == "fail" {
			return fmt.Errorf("injected startup failure")
		}
		return emit("sample/start", "SUCCEEDED", "PID 456 is running; readiness not checked", 1, 1)
	}
	g = grpc.NewServer()
	s.Register(g)
	l.endpoint = serve(t, g, &api.Capabilities{Server: "juno", ApiVersion: 2, PackageId: "host:default", Features: s.Features()})
	target = &api.Target{PackageId: "host:default", Endpoint: l.endpoint, Group: "test"}
	password, _ := cipher.Aes256Encode("test")
	l.cfg = config.JupiterContextRecord{Jupiter: endpoint, User: "operator", Password: password}
	t.Cleanup(func() { s.Close(); j.Close(); c.Close() })
	return l
}
func TestCronGRPCControl(t *testing.T) {
	l := newControlLab(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rocron", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	catalog, err := c.Cron(ctx)
	if err != nil || len(catalog.Jobs) != 1 {
		t.Fatal(catalog, err)
	}
	q := &api.CronRequest{RequestId: "test-request", Process: "sample", Job: "run"}
	op, err := c.RunCron(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := c.Watch(ctx, "rocron", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "REQUESTED" {
		t.Fatal(op)
	}
	_, err = c.RunCron(ctx, q)
	if err != nil || l.calls.Load() != 1 {
		t.Fatal(l.calls.Load(), err)
	}
	l.cfg.User = "monitor"
	m, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rocron", Package: "host:default"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	_, err = m.RunCron(ctx, q)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
	q.RequestId = "missing-job"
	q.Job = "missing"
	op, err = c.RunCron(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ = c.Watch(ctx, "rocron", op.Id)
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "FAILED" || l.calls.Load() != 1 {
		t.Fatal(op)
	}
	_, err = controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "unknown-feature", Package: "host"})
	if err != transport.ErrLegacy {
		t.Fatal(err)
	}
}
