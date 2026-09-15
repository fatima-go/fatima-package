#!/usr/bin/env python3
"""Verify released v1/v2 IPC and OPM wire compatibility in isolated processes.
No running Fatima installation, user configuration or production endpoint is used.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import time

ENV = dict(os.environ, GOWORK="off")
IPC = r'''
package main
import (
 "context"
 "fmt"
 "os"
 "os/signal"
 "path/filepath"
 "strconv"
 "syscall"
 "time"
 fatima "CORE"
 "CORE/ipc"
 EXTRA_IMPORT
)
type rt struct{ fatima.FatimaRuntime; root, name string }
func(r rt)GetEnv()fatima.FatimaEnv{return env{root:r.root,name:r.name}}
type env struct{fatima.FatimaEnv;root,name string}
func(e env)GetFolderGuide()fatima.FolderGuide{return folders{root:e.root,name:e.name}}
func(e env)GetSystemProc()fatima.SystemProc{return process{name:e.name}}
type folders struct{fatima.FolderGuide;root,name string}
func(f folders)GetFatimaHome()string{return f.root}
func(f folders)GetAppProcFolder()string{return filepath.Join(f.root,"app",f.name,"proc")}
type process struct{fatima.SystemProc;name string}
func(p process)GetProgramName()string{return p.name}
func(p process)GetPid()int{return os.Getpid()}
type drain struct{root string}
func(d drain)Goaway(){os.WriteFile(filepath.Join(d.root,"drained"),[]byte("done"),0600)}
func main(){
 root,name:=os.Args[1],os.Args[2]
 dir:=filepath.Join(root,"app",name,"proc");os.MkdirAll(dir,0700)
 os.WriteFile(filepath.Join(dir,name+".pid"),[]byte(strconv.Itoa(os.Getpid())),0600)
 closer:=ipc.StartIPCService(rt{root:root,name:name},nil,drain{root},func(string,[]string){})
 defer closer.Close()
 _=context.Background();_=fmt.Sprint;_=time.Second
 ACTION
}
'''
WIRE = r'''
package main
import (
 "context"
 "fmt"
 "net"
 "os"
 "time"
 "OPM/api"
 "google.golang.org/grpc"
 "google.golang.org/grpc/credentials/insecure"
)
type server struct{api.UnimplementedDeploymentsServer}
func(server)Get(context.Context,*api.RolloutQuery)(*api.Rollout,error){return &api.Rollout{Id:"existing",State:"RUNNING",ManagementSessionId:"session",NextTargetStartAt:12345,Targets:[]*api.TargetRun{{Operation:&api.Operation{Id:"op",State:"SUCCEEDED"}}}},nil}
func main(){
 if os.Args[1]=="server"{
  l,e:=net.Listen("tcp","127.0.0.1:0");if e!=nil{panic(e)}
  g:=grpc.NewServer();api.RegisterDeploymentsServer(g,server{})
  os.WriteFile(os.Args[2],[]byte(l.Addr().String()),0600);g.Serve(l);return
 }
 c,e:=grpc.NewClient(os.Args[2],grpc.WithTransportCredentials(insecure.NewCredentials()));if e!=nil{panic(e)};defer c.Close()
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 p,e:=api.NewDeploymentsClient(c).Get(ctx,&api.RolloutQuery{Id:"existing"})
 if e!=nil || p.Id!="existing" || p.ManagementSessionId!="session" || p.NextTargetStartAt!=12345 || p.Targets[0].Operation.State!="SUCCEEDED"{panic(fmt.Sprint(p,e))}
}
'''

def build(root, name, source, requires):
    folder = root / name
    folder.mkdir()
    (folder / "go.mod").write_text("module compatibility/" + name + "\n\ngo 1.25.0\n\nrequire (\n" + requires + "\n)\n")
    (folder / "main.go").write_text(source)
    subprocess.run(["go", "mod", "tidy"], cwd=folder, env=ENV, check=True, capture_output=True)
    output = folder / "probe"
    subprocess.run(["go", "build", "-o", str(output), "."], cwd=folder, env=ENV, check=True)
    return output


def ready(path, process):
    for _ in range(100):
        if path.exists():
            return
        if process.poll() is not None:
            raise RuntimeError("compatibility server exited")
        time.sleep(.05)
    raise RuntimeError("compatibility server startup timeout")


def main():
    # Short path keeps Unix socket names below the macOS length limit.
    with tempfile.TemporaryDirectory(prefix="fcompat-", dir="/tmp") as folder:
        root = Path(folder)
        for version in ("v1.3.4", "v1.3.7"):
            source = IPC.replace("CORE", "github.com/fatima-go/fatima-core").replace("EXTRA_IMPORT", "")
            source = source.replace("ACTION", 'done:=make(chan os.Signal,1);signal.Notify(done,syscall.SIGTERM);<-done')
            old = build(root, "old" + version.replace(".", ""), source, "github.com/fatima-go/fatima-core " + version)
            source = IPC.replace("CORE", "github.com/fatima-go/fatima-core/v2").replace("EXTRA_IMPORT", '"github.com/fatima-go/juno/service/goaway"')
            source = source.replace("ACTION", 'ipc.RegisterIPCSessionListener(goaway.NewGoawayManager());_=signal.Notify;_=syscall.SIGTERM;ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();if e:=goaway.ExecuteObserved(ctx,"sample",func(string,int64)error{return nil});e!=nil{panic(e)}')
            new = build(root, "new" + version.replace(".", ""), source, "github.com/fatima-go/fatima-core/v2 v2.0.0\ngithub.com/fatima-go/juno v0.0.0-20260915054052-1fad0a2f7447")
            home = root / ("home" + version)
            home.mkdir()
            with (root / (version + ".log")).open("w") as log:
                child = subprocess.Popen([old, home, "sample"], stdout=log, stderr=log)
                try:
                    ready(home / "app/sample/proc" / ("fatima.sample." + str(child.pid) + ".sock"), child)
                    subprocess.run([new, home, "juno"], check=True, stdout=log, stderr=log, timeout=15)
                    assert (home / "drained").read_text() == "done"
                    child.terminate()
                    assert child.wait(timeout=5) == 0
                except Exception:
                    log.flush()
                    print((root / (version + ".log")).read_text())
                    raise
                finally:
                    if child.poll() is None:
                        child.kill(); child.wait()
            print("PASS new Juno goaway/IPC -> core", version, flush=True)
        binaries = {}
        for label, module, requires in (
            ("oldwire", "github.com/fatima-go/fatima-core/opm", "github.com/fatima-go/fatima-core v1.3.7"),
            ("newwire", "github.com/fatima-go/fatima-opm", "github.com/fatima-go/fatima-opm v1.0.0"),
        ):
            binaries[label] = build(root, label, WIRE.replace("OPM", module), requires)
        for server, client in (("oldwire", "newwire"), ("newwire", "oldwire")):
            address = root / (server + ".address")
            child = subprocess.Popen([binaries[server], "server", address])
            try:
                ready(address, child)
                subprocess.run([binaries[client], "client", address.read_text()], check=True, timeout=15)
            finally:
                child.terminate(); child.wait(timeout=5)
            print("PASS", client, "->", server, flush=True)

if __name__ == "__main__":
    main()
