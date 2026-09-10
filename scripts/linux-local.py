#!/usr/bin/env python3
"""Persistent Colima Linux arm64 test package, reachable by Mac CLI on 9190/9180."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tarfile
import time
import urllib.request
import zipfile

BASE = Path(__file__).resolve().parents[1]
LAB = BASE / ".local/progressive-deploy/linux-operations"
NAME = "fatima-operations-local"
IMAGE = "fatima-operations-linux:local"
VOLUME = "fatima-operations-home"
TARGET = "linux01:default"
MAC_HOME = Path(os.environ.get("FATIMA_HOME", "/missing-fatima-home"))
SOCKET = Path.home() / ".colima/default/docker.sock"
DOCKER_ENV = dict(os.environ, DOCKER_HOST="unix://" + str(SOCKET), DOCKER_BUILDKIT="0")
DOCKER_ENV.pop("DOCKER_CONTEXT", None)


def call(args, **kwargs):
    return subprocess.check_output([str(x) for x in args], text=True, **kwargs).strip()


def docker(*args):
    return call(["docker", *args], env=DOCKER_ENV)


def inside(*args):
    return docker("exec", NAME, *args)


def local_guard():
    context = call([MAC_HOME / "bin/rocontext"])
    if not re.search(r"^\*\s+local\s+http://127\.0\.0\.1:9190\s", context, re.M):
        raise RuntimeError("select rocontext use local (127.0.0.1:9190) first")


def prepare():
    archive = BASE.parent / "fatima-download/fatima-package.linux-arm64.tar.gz"
    with tarfile.open(archive) as tar:
        info = json.load(tar.extractfile("fatima-package/packing-info.json"))
    if (info["os"], info["architecture"]) != ("linux", "arm64"):
        raise RuntimeError("a real Linux arm64 package is required")
    context = LAB / "image"
    fixture = context / "fixture"
    fixture.mkdir(parents=True, exist_ok=True)
    shutil.copy2(archive, context / archive.name)
    for name in ("Dockerfile", "entrypoint.sh"):
        shutil.copy2(BASE / "integration/linux" / name, context / name)

    def write(path, value):
        dest = fixture / path
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(value)

    write("conf/fatima-package.yaml", """# Writable Linux integration fixture. Registry changes must preserve this comment.
group:
  - {id: 1, name: OPM}
  - {id: 4, name: SVC}
process:
  - {gid: 1, name: jupiter, loglevel: info, startmode: 1}
  - {gid: 1, name: juno, loglevel: info, startmode: 1}
  - {gid: 1, name: saturn, loglevel: info, startmode: 1}
  - {gid: 4, name: ctlprobe, loglevel: info, startmode: 1}
  - {gid: 4, name: dpv2, loglevel: info, startmode: 1}
""")
    write("conf/fatima-package-predefine.properties", "var.global.package.groupname=linux-deploy\nvar.global.package.hostname=linux01\nvar.global.package.name=default\nvar.host.ipaddress=127.0.0.1\n")
    write("app/jupiter/application.properties", "repo=file\nwebserver.address=127.0.0.1\nwebserver.port=9190\n")
    write("app/juno/application.properties", "gateway.address=127.0.0.1\ngateway.port=9190\nwebserver.address=127.0.0.1\nwebserver.port=9180\n")
    write("app/saturn/application.properties", "message.notify.chain=disabled\n")
    write("app/ctlprobe/application.properties", "cron.control.probe.spec=0 0 0 1 1 *\ncron.control.probe.desc=Linux control API test\ncron.control.probe.sample=probe-message\ncron.control.probe.primary=false\ngofatima.component.lifecycle.timeout.seconds=5\n")
    env = dict(os.environ, GOWORK=str(BASE / "go.work.dev"), GOOS="linux", GOARCH="arm64", CGO_ENABLED="0")
    subprocess.run(["go", "build", "-trimpath", "-o", str(fixture / "app/ctlprobe/ctlprobe"), "./integration/testdata/controlsample"], cwd=BASE, env=env, check=True)
    # A reproducible multi-platform FAR from the checked-in sample, independent
    # of any developer project or files left over from a previous test run.
    artifact = LAB / "artifact"
    artifact.mkdir(exist_ok=True)
    for system, arch in (("darwin", "arm64"), ("linux", "arm64"), ("linux", "amd64")):
        binary = artifact / "platform" / (system + "_" + arch) / "dpv2"
        binary.parent.mkdir(parents=True, exist_ok=True)
        build_env = dict(env, GOOS=system, GOARCH=arch)
        subprocess.run(["go", "build", "-trimpath", "-o", str(binary), "./integration/testdata/deploysample"], cwd=BASE, env=build_env, check=True)
    (artifact / "deployment.json").write_text(json.dumps({"process": "dpv2", "process_type": "GENERAL", "build": {"user": "linux-local-test", "git": {"commit": call(["git", "rev-parse", "HEAD"], cwd=BASE)}, "time": time.strftime("%Y-%m-%d %H:%M:%S %Z")}}))
    (artifact / "application.properties").write_text("gofatima.component.lifecycle.timeout.seconds=5\n")
    with zipfile.ZipFile(LAB / "dpv2.far", "w", zipfile.ZIP_DEFLATED) as far:
        for file in sorted(artifact.rglob("*")):
            if file.is_file():
                far.write(file, "/" + file.relative_to(artifact).as_posix())
    subprocess.run(["docker", "build", "-t", IMAGE, str(context)], env=DOCKER_ENV, check=True)
    (LAB / "package.json").write_text(json.dumps({"archive": str(archive), "sha256": hashlib.sha256(archive.read_bytes()).hexdigest(), "packing_info": info}, indent=2) + "\n")


def stop_mac():
    local_guard()
    pids = []
    for name in ("jupiter", "juno", "saturn"):
        path = MAC_HOME / "app" / name / "proc" / (name + ".pid")
        if path.exists() and path.read_text().strip().isdigit():
            pid = int(path.read_text().strip())
            process = subprocess.run(["ps", "-p", str(pid), "-o", "comm="], capture_output=True, text=True)
            if process.returncode == 0:
                if Path(process.stdout.strip()).name != name:
                    raise RuntimeError(f"stale {name} PID now belongs to another process; refusing to signal it")
                pids.append(pid)
    # Do not feed stale PID files to stopro when the Mac OPM is already down.
    for pid in pids:
        os.kill(pid, 15)
    until = time.monotonic() + 45
    while pids:
        pids = [pid for pid in pids if subprocess.run(["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True).returncode == 0]
        if time.monotonic() >= until:
            raise RuntimeError("Mac OPM did not stop")
        if pids:
            time.sleep(.2)


def wait_ready():
    until = time.monotonic() + 40
    while time.monotonic() < until:
        try:
            with urllib.request.urlopen("http://127.0.0.1:9180/.well-known/fatima/capabilities", timeout=2) as response:
                caps = json.load(response)
            if caps.get("package_id") == TARGET:
                listing = json.loads(call([MAC_HOME / "bin/ropack", "--json"]))
                if any(p["target"]["package_id"] == TARGET and p["state"] == "ALIVE" for p in listing.get("packages", [])):
                    return
        except (OSError, ValueError, subprocess.CalledProcessError):
            pass
        time.sleep(.5)
    raise RuntimeError("Mac cannot reach Linux Jupiter/Juno on 9190/9180")


def up():
    local_guard()
    if not SOCKET.exists():
        subprocess.run(["colima", "start", "--activate=false", "--save-config=false"], check=True)
    platform = docker("info", "--format", "{{.OSType}}/{{.Architecture}}")
    if platform != "linux/aarch64":
        raise RuntimeError("expected native Linux arm64 Docker, got " + platform)
    existing = docker("ps", "-a", "--filter", "name=^/" + NAME + "$", "--format", "{{.Names}}")
    if existing:
        if docker("inspect", NAME, "--format", "{{.Config.Labels.fatima_test}}") != "linux-operations":
            raise RuntimeError("container name belongs to another task")
        docker("stop", "--time", "15", NAME)
        docker("rm", NAME)
    prepare()
    stop_mac()
    try:
        # In Colima this is the Linux VM network, not the Mac network. Its
        # automatic localhost forwarding preserves the registered loopback URI.
        # HTTP and native gRPC traverse the same two TCP ports without a proxy.
        docker("run", "-d", "--init", "--name", NAME, "--network", "host", "--label", "fatima_test=linux-operations", "--mount", "type=volume,src=" + VOLUME + ",dst=/fatima", IMAGE)
        wait_ready()
    except Exception:
        subprocess.run(["docker", "stop", "--time", "15", NAME], env=DOCKER_ENV, capture_output=True)
        subprocess.run([str(MAC_HOME / "bin/startro"), "-y"], check=True, start_new_session=True)
        raise
    print(docker("exec", NAME, "uname", "-a"))
    print(call([MAC_HOME / "bin/ropack", "--plain"]))


def down(restore):
    local_guard()
    if docker("ps", "--filter", "name=^/" + NAME + "$", "--format", "{{.Names}}"):
        if docker("inspect", NAME, "--format", "{{.Config.Labels.fatima_test}}") != "linux-operations":
            raise RuntimeError("container name belongs to another task")
        docker("stop", "--time", "15", NAME)
    if restore:
        until = time.monotonic() + 15
        while time.monotonic() < until:
            try:
                with socket.create_connection(("127.0.0.1", 9190), timeout=.2):
                    time.sleep(.2)
            except OSError:
                break
        subprocess.run([str(MAC_HOME / "bin/startro"), "-y"], check=True, start_new_session=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("up", "down", "restore-mac", "status"))
    action = parser.parse_args().action
    if action == "up":
        up()
    elif action == "status":
        local_guard()
        print(docker("ps", "--filter", "name=^/" + NAME + "$"))
        print(call([MAC_HOME / "bin/ropack", "--plain"]))
    else:
        down(action == "restore-mac")
