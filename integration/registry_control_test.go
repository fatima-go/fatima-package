package integration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-cmd/controlui"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/operations"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestRegistryGatewayForwarding(t *testing.T) {
	l := newControlLab(t)
	var revision atomic.Int32
	revision.Store(1)
	l.server.RegistryCatalog = func(*api.RegistryQuery) (*api.RegistryCatalog, error) {
		c, err := l.server.Processes(&api.ProcessQuery{})
		return &api.RegistryCatalog{Catalog: c, Groups: []*api.ProcessGroup{{Id: 4, Name: "SVC"}}, Revision: fmt.Sprint(revision.Load())}, err
	}
	l.server.RegistryPreview = func(q *api.RegistryRequest) (*api.RegistryPlan, error) {
		r := proto.Clone(q).(*api.RegistryRequest)
		r.ExpectedRevision = fmt.Sprint(revision.Load())
		return &api.RegistryPlan{Request: r, Effects: []string{"registration"}}, nil
	}
	l.server.RegistryApply = func(ctx context.Context, q *api.RegistryRequest, emit operations.Emit) error {
		if q.ExpectedRevision != fmt.Sprint(revision.Load()) {
			return fmt.Errorf("configuration changed")
		}
		l.calls.Add(1)
		if err := emit("registration", "RUNNING", "Writing registration", 0, 1); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		return emit("registration", "SUCCEEDED", "Registration saved", 1, 1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "roproc", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	o := controlui.Options{Command: "roproc", Action: "add", Process: "newproc", RegistryGroup: "4", RequestID: "registry-001"}
	o.Plan, err = c.PreviewRegistry(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	op, err := c.Submit(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	watchCtx, stopWatch := context.WithCancel(ctx)
	stream, err := c.Watch(watchCtx, "roproc", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatal(err)
	}
	stopWatch()
	stream, err = c.Watch(ctx, "roproc", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "SUCCEEDED" {
		t.Fatal(op)
	}
	if _, err = c.Submit(ctx, o); err != nil || l.calls.Load() != 1 {
		t.Fatal("duplicate operation", l.calls.Load(), err)
	}
	revision.Store(2)
	o.RequestID = "registry-stale"
	o.Plan.Request.RequestId = o.RequestID
	op, err = c.Submit(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	stream, err = c.Watch(ctx, "roproc", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "FAILED" || l.calls.Load() != 1 {
		t.Fatal(op, l.calls.Load())
	}
	l.cfg.User = "monitor"
	m, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "roproc", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err = m.Submit(ctx, o); status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
	// Direct mismatched-package attempts are rejected by Juno too.
	backend, _ := transport.Dial(l.endpoint)
	defer backend.Close()
	auth, _ := c.Context(ctx)
	_, err = api.NewProcessRegistryClient(backend).Catalog(auth, &api.RegistryQuery{PackageId: "wrong:default"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
}
