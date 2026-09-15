package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-cmd/controlui"
	"github.com/fatima-go/fatima-opm/api"
)

func TestPackageInventoryMixedPeers(t *testing.T) {
	l := newControlLab(t)
	var probes atomic.Int32
	var generation atomic.Int32
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/package/health/v1" && r.Method == "POST" {
			probes.Add(1)
			w.WriteHeader(200)
			return
		}
		http.NotFound(w, r)
	}))
	defer old.Close()
	l.gateway.Inventory = func() ([]*api.PackageEntry, error) {
		group := "first"
		if generation.Load() > 0 {
			group = "second"
		}
		return []*api.PackageEntry{
			{Target: &api.Target{PackageId: "host:default", Endpoint: l.endpoint, Group: group, Platform: "darwin_arm64"}, RegisteredAt: 123},
			{Target: &api.Target{PackageId: "old:default", Endpoint: old.URL, Group: "old"}},
			{Target: &api.Target{PackageId: "wrong:default", Endpoint: l.endpoint, Group: "wrong"}},
		}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	l.cfg.User = "monitor"
	c, err := controlui.Connect(ctx, l.cfg, "test", controlui.Options{Command: "ropack"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	v, err := c.Packages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, p := range v.Packages {
		states[p.Target.PackageId] = p.State
	}
	if states["host:default"] != "ALIVE" || states["old:default"] != "ALIVE" || states["wrong:default"] != "MISMATCH" {
		t.Fatal(v)
	}
	stream, err := c.WatchPackages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatal(err)
	}
	if probes.Load() != 1 {
		t.Fatal("shared probe cache missing", probes.Load())
	}
	generation.Store(1)
	for {
		v, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		changed := false
		for _, p := range v.Packages {
			if p.Target.PackageId == "host:default" && p.Target.Group == "second" {
				changed = true
			}
		}
		if changed {
			break
		}
	}
	cancel()
	if _, err = stream.Recv(); err == nil {
		t.Fatal("canceled subscription stayed open")
	}
}
