package integration

import (
	"context"
	"github.com/fatima-go/fatima-cmd/controlui"
	"github.com/fatima-go/fatima-core/opm/operations"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"testing"
	"time"
)

func TestStartControl(t *testing.T) {
	l := newControlLab(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rostart", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	o := controlui.Options{Command: "rostart", RequestID: "start-1", Targets: []string{"sample"}}
	op, err := c.Submit(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ := c.Watch(ctx, "rostart", op.Id)
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "SUCCEEDED" || !strings.Contains(op.Message, "readiness") {
		t.Fatal(op)
	}
	if _, err = c.Submit(ctx, o); err != nil || l.calls.Load() != 1 {
		t.Fatal(l.calls.Load(), err)
	}
	l.cfg.User = "monitor"
	m, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rostart", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err = m.Submit(ctx, o); status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
}
