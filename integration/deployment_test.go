package integration

import (
	"archive/zip"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"github.com/fatima-go/fatima-opm/transport"
	juno "github.com/fatima-go/juno/deployment"
	jupiter "github.com/fatima-go/jupiter/deployment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func serve(t *testing.T, g *grpc.Server, caps *api.Capabilities) string {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := transport.NewServer(&http.Server{Handler: http.NotFoundHandler()}, g, caps)
	go s.Serve(l)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return "http://" + l.Addr().String()
}
func createFAR(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "example.far")
	f, _ := os.Create(path)
	z := zip.NewWriter(f)
	w, _ := z.Create("deployment.json")
	w.Write([]byte(`{"process":"example","build":{"author":"builder"}}`))
	w, _ = z.Create("platform/darwin_arm64/example")
	w.Write([]byte("#!/bin/sh\nexit 0\n"))
	z.Close()
	f.Close()
	b, _ := os.ReadFile(path)
	return b
}

type lab struct {
	jup   *jupiter.Server
	conn  *grpc.ClientConn
	ctx   context.Context
	calls [3]atomic.Int32
	fail  atomic.Bool
}

func newLab(t *testing.T) *lab {
	l := &lab{}
	var targets []*api.Target
	s, e := jupiter.New(t.TempDir(), func(u, p string) (string, error) {
		if u == "operator" && p == "local-test" {
			return "OPERATOR", nil
		}
		if u == "monitor" && p == "local-test" {
			return "MONITOR", nil
		}
		return "", fmt.Errorf("bad login")
	}, func() ([]*api.Target, error) { return targets, nil })
	if e != nil {
		t.Fatal(e)
	}
	l.jup = s
	g := grpc.NewServer()
	s.Register(g)
	endpoint := serve(t, g, s.Capabilities())
	conn, e := transport.Dial(endpoint)
	if e != nil {
		t.Fatal(e)
	}
	l.conn = conn
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("host%d:default", i+1)
		auth := func(ctx context.Context, q *api.ValidateRequest) error {
			_, e := api.NewIdentityClient(conn).Validate(transport.WithToken(ctx, transport.Token(ctx)), q)
			return e
		}
		executor := func(ctx context.Context, spec *api.OperationSpec, path string, emit juno.Emit) (juno.Result, error) {
			l.calls[i].Add(1)
			sum, n, e := artifact.Digest(path)
			if e != nil || sum != spec.Sha256 || n != spec.Size {
				return juno.Result{}, fmt.Errorf("incorrect FAR")
			}
			for _, stage := range []string{"goaway", "shutdown", "install", "start"} {
				if e = emit(stage, "RUNNING", stage+" begins", 0, 1); e != nil {
					return juno.Result{}, e
				}
				select {
				case <-ctx.Done():
					return juno.Result{}, ctx.Err()
				case <-time.After(40 * time.Millisecond):
				}
				if e = emit(stage, "SUCCEEDED", stage+" complete", 1, 1); e != nil {
					return juno.Result{}, e
				}
			}
			if i == 1 && l.fail.Load() {
				return juno.Result{}, fmt.Errorf("injected start failure")
			}
			return juno.Result{RevisionPath: "test-revision"}, nil
		}
		j, e := juno.New(t.TempDir(), id, "darwin_arm64", auth, executor)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(j.Close)
		g := grpc.NewServer()
		j.Register(g)
		address := serve(t, g, j.Capabilities())
		targets = append(targets, &api.Target{PackageId: id, Group: "backend01", Endpoint: address})
	}
	session, e := api.NewIdentityClient(conn).Login(context.Background(), &api.LoginRequest{Username: "operator", Password: "local-test"})
	if e != nil {
		t.Fatal(e)
	}
	l.ctx = transport.WithToken(context.Background(), session.Token)
	s.StartWorkers()
	t.Cleanup(func() { s.Close(); conn.Close() })
	return l
}
func upload(t *testing.T, l *lab, key string) *api.Artifact {
	t.Helper()
	data := createFAR(t)
	path := filepath.Join(t.TempDir(), "far")
	os.WriteFile(path, data, 0600)
	hash, n, _ := artifact.Digest(path)
	stream, e := api.NewArtifactsClient(l.conn).Upload(l.ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = stream.Send(&api.UploadChunk{Header: &api.UploadHeader{RequestId: key, Filename: "example.far", Size: n, Sha256: hash}}); e != nil {
		t.Fatal(e)
	}
	if e = stream.Send(&api.UploadChunk{Data: data}); e != nil {
		t.Fatal(e)
	}
	a, e := stream.CloseAndRecv()
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func await(t *testing.T, l *lab, id, state string) *api.Rollout {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		p, e := api.NewDeploymentsClient(l.conn).Get(l.ctx, &api.RolloutQuery{Id: id})
		if e != nil {
			t.Fatal(e)
		}
		if p.State == state {
			return p
		}
		if p.State == "ATTENTION" || p.State == "FAILED" || p.State == "EXPIRED" {
			t.Fatalf("wanted %s, got %s: %s", state, p.State, p.Message)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + state)
	return nil
}
func TestThreePackageRolloutUploadOncePauseReconnectContinue(t *testing.T) {
	l := newLab(t)
	a := upload(t, l, "one-upload")
	if a.ExpiresAt-a.UploadedAt != 86400 {
		t.Fatal("incorrect TTL")
	}
	same := upload(t, l, "one-upload")
	if same.Id != a.Id {
		t.Fatal("idempotent upload created another artifact")
	}
	d := api.NewDeploymentsClient(l.conn)
	q := &api.CreateRollout{RequestId: "rollout-1", ArtifactId: a.Id, Group: "backend01", FirstPackageId: "host1:default"}
	p, e := d.Create(l.ctx, q)
	if e != nil {
		t.Fatal(e)
	}
	duplicate, e := d.Create(l.ctx, q)
	if e != nil || duplicate.Id != p.Id {
		t.Fatal("create not idempotent")
	}
	ctx, cancel := context.WithCancel(l.ctx)
	watch, e := d.Watch(ctx, &api.WatchRequest{Id: p.Id})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = watch.Recv(); e != nil {
		t.Fatal(e)
	}
	cancel() // Disconnecting the CLI must not cancel execution.
	p = await(t, l, p.Id, "WAITING")
	if l.calls[0].Load() != 1 || l.calls[1].Load() != 0 || l.calls[2].Load() != 0 {
		t.Fatal("first-package gate was bypassed")
	}
	if len(p.Targets[0].Operation.Events) < 8 {
		t.Fatal("detailed progress missing")
	}
	if _, e = d.Act(l.ctx, &api.RolloutAction{Id: p.Id, Action: "continue", ExpectedRevision: p.Revision - 1}); status.Code(e) != codes.Aborted {
		t.Fatal("stale approval accepted")
	}
	p, e = d.Act(l.ctx, &api.RolloutAction{Id: p.Id, Action: "continue", ExpectedRevision: p.Revision})
	if e != nil {
		t.Fatal(e)
	}
	p = await(t, l, p.Id, "SUCCEEDED")
	for i, run := range p.Targets {
		if run.Operation.Sha256 != a.Sha256 || l.calls[i].Load() != 1 {
			t.Fatal("artifact changed or repeated execution")
		}
	}
	list, e := api.NewArtifactsClient(l.conn).List(l.ctx, &api.ArtifactQuery{})
	if e != nil || len(list.Artifacts) != 1 {
		t.Fatal("remaining rollout uploaded again")
	}
}
func TestFailureStopsRemainingAndRequiresExplicitRetry(t *testing.T) {
	l := newLab(t)
	l.fail.Store(true)
	a := upload(t, l, "retry-upload")
	d := api.NewDeploymentsClient(l.conn)
	p, e := d.Create(l.ctx, &api.CreateRollout{RequestId: "retry-plan", ArtifactId: a.Id, Group: "backend01", FirstPackageId: "host1:default"})
	if e != nil {
		t.Fatal(e)
	}
	p = await(t, l, p.Id, "WAITING")
	p, e = d.Act(l.ctx, &api.RolloutAction{Id: p.Id, Action: "continue", ExpectedRevision: p.Revision})
	if e != nil {
		t.Fatal(e)
	}
	p = await(t, l, p.Id, "FAILED")
	if l.calls[2].Load() != 0 {
		t.Fatal("continued beyond failure")
	}
	l.fail.Store(false)
	p, e = d.Act(l.ctx, &api.RolloutAction{Id: p.Id, Action: "retry", ExpectedRevision: p.Revision})
	if e != nil {
		t.Fatal(e)
	}
	p = await(t, l, p.Id, "SUCCEEDED")
	if l.calls[0].Load() != 1 || l.calls[1].Load() != 2 || len(p.Targets[1].PreviousAttempts) != 1 {
		t.Fatal("retry repeated a successful target or lost audit")
	}
}
func TestAuthorizationAndConflict(t *testing.T) {
	l := newLab(t)
	a := upload(t, l, "conflict-upload")
	d := api.NewDeploymentsClient(l.conn)
	if _, e := d.List(context.Background(), &api.Empty{}); status.Code(e) != codes.Unauthenticated {
		t.Fatal("missing authentication accepted")
	}
	session, e := api.NewIdentityClient(l.conn).Login(context.Background(), &api.LoginRequest{Username: "monitor", Password: "local-test"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Create(transport.WithToken(context.Background(), session.Token), &api.CreateRollout{}); status.Code(e) != codes.PermissionDenied {
		t.Fatal("monitor allowed mutation")
	}
	q := &api.CreateRollout{RequestId: "owner", ArtifactId: a.Id, Group: "backend01", FirstPackageId: "host1:default"}
	if _, e = d.Create(l.ctx, q); e != nil {
		t.Fatal(e)
	}
	q.RequestId = "conflict"
	if _, e = d.Create(l.ctx, q); status.Code(e) != codes.Aborted {
		t.Fatal("two active plans accepted")
	}
	q.RequestId = "owner"
	q.FirstPackageId = "host2:default"
	if _, e = d.Create(l.ctx, q); status.Code(e) != codes.AlreadyExists {
		t.Fatal("request ID reused with different payload")
	}
}

func TestCorruptUploadsNeverBecomeArtifactsOrOperations(t *testing.T) {
	l := newLab(t)
	data := createFAR(t)
	u, e := api.NewArtifactsClient(l.conn).Upload(l.ctx)
	if e != nil {
		t.Fatal(e)
	}
	u.Send(&api.UploadChunk{Header: &api.UploadHeader{RequestId: "corrupt", Filename: "test.far", Size: int64(len(data)), Sha256: fmt.Sprintf("%064d", 0)}})
	u.Send(&api.UploadChunk{Data: data})
	if _, e = u.CloseAndRecv(); status.Code(e) != codes.DataLoss {
		t.Fatalf("corrupt artifact accepted: %v", e)
	}
	list, e := api.NewArtifactsClient(l.conn).List(l.ctx, &api.ArtifactQuery{})
	if e != nil || len(list.Artifacts) != 0 {
		t.Fatal("corrupt upload was committed")
	}
	var executions atomic.Int32
	j, e := juno.New(t.TempDir(), "test:default", "darwin_arm64", func(context.Context, *api.ValidateRequest) error { return nil }, func(context.Context, *api.OperationSpec, string, juno.Emit) (juno.Result, error) {
		executions.Add(1)
		return juno.Result{}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	g := grpc.NewServer()
	j.Register(g)
	endpoint := serve(t, g, j.Capabilities())
	conn, e := transport.Dial(endpoint)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	stream, e := api.NewPackageDeploymentClient(conn).Stage(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	stream.Send(&api.StageChunk{Spec: &api.OperationSpec{Id: "corrupt", ArtifactId: "artifact", Process: "example", PackageId: "test:default", Size: int64(len(data)), Sha256: fmt.Sprintf("%064d", 0), ExpiresAt: time.Now().Add(time.Hour).Unix()}})
	stream.Send(&api.StageChunk{Data: data})
	if _, e = stream.CloseAndRecv(); status.Code(e) != codes.DataLoss {
		t.Fatalf("corrupt Juno staging accepted: %v", e)
	}
	if _, e = api.NewPackageDeploymentClient(conn).Start(context.Background(), &api.OperationQuery{Id: "corrupt"}); status.Code(e) != codes.NotFound {
		t.Fatal("corrupt operation persisted")
	}
	if executions.Load() != 0 {
		t.Fatal("corruption reached the executor")
	}
}
