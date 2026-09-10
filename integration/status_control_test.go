package integration

import (
	"context"
	"github.com/fatima-go/fatima-cmd/controlui"
	"testing"
	"time"
)

func TestProcessStatusSubscription(t *testing.T) {
	l := newControlLab(t)
	l.cfg.User = "monitor"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "rodis", Package: "host"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	stream, err := c.WatchProcesses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v, err := stream.Recv()
	if err != nil || v.Processes[0].State != "ALIVE" {
		t.Fatal(v, err)
	}
	l.pid.Store(0)
	v, err = stream.Recv()
	if err != nil || v.Processes[0].State != "DEAD" {
		t.Fatal(v, err)
	}
	cancel()
	if _, err = stream.Recv(); err == nil {
		t.Fatal("subscription did not cancel")
	}
}
