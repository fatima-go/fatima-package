package integration

import (
	"context"
	"github.com/fatima-go/fatima-cmd/controlui"
	"github.com/fatima-go/fatima-opm/operations"
	"testing"
	"time"
)

func TestStopStreamingAndReconnect(t *testing.T) {
	l := newControlLab(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rostop", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	catalog, err := c.Processes(ctx)
	if err != nil || len(catalog.Processes) != 1 {
		t.Fatal(catalog, err)
	}
	o := controlui.Options{Command: "rostop", RequestID: "stop-1", Targets: []string{"sample"}}
	op, err := c.Submit(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	disconnected, stop := context.WithCancel(ctx)
	stream, err := c.Watch(disconnected, "rostop", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatal(err)
	}
	stop()
	stream, err = c.Watch(ctx, "rostop", op.Id)
	if err != nil {
		t.Fatal(err)
	}
	stages := map[string]bool{}
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range op.Events {
			stages[e.Stage] = true
		}
	}
	if op.State != "SUCCEEDED" || !stages["sample/goaway"] || !stages["sample/shutdown"] {
		t.Fatal(op)
	}
	if _, err = c.Submit(ctx, o); err != nil || l.calls.Load() != 1 {
		t.Fatal(l.calls.Load(), err)
	}
	o.RequestID = "stop-fail"
	o.Targets = []string{"fail"}
	op, err = c.Submit(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ = c.Watch(ctx, "rostop", op.Id)
	for !operations.Terminal(op.State) {
		op, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
	}
	if op.State != "FAILED" {
		t.Fatal(op)
	}
}
