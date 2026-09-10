#!/usr/bin/env python3
"""Three real Juno processes, isolated Linux PID namespaces, existing OPM ports."""
import json
import os
from pathlib import Path
import subprocess
import time

base = Path(__file__).resolve().parents[1]
lab = base / ".local/progressive-deploy"
network = "fatima-v2-test"
names = ["fatima-v2-jupiter"] + [f"fatima-v2-juno{i}" for i in range(1, 4)]


def command(args, **kw):
    return subprocess.check_output(args, text=True, **kw).strip()


def inside(name, args):
    return command(["docker", "exec", name, *args])


def cli(*args):
    return json.loads(inside(names[0], ["rodeploy", "--json", *args]))


def wait(id, state, seconds=40):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        p = cli("watch", id)
        if p["state"] == state:
            return p
        if p["state"] in ("FAILED", "ATTENTION", "EXPIRED"):
            raise RuntimeError(json.dumps(p))
        time.sleep(0.5)
    raise RuntimeError("timeout awaiting " + state)


try:
    subprocess.run(["docker", "network", "create", network], check=True, stdout=subprocess.DEVNULL)
    for i, name in enumerate(names):
        root = lab / "linux" / name
        root.mkdir(exist_ok=True)
        (root / "fatima-package.yaml").write_text("""group:
  - {id: 1, name: OPM}
  - {id: 4, name: SVC}
process:
  - {gid: 1, name: jupiter, loglevel: info, startmode: 1}
  - {gid: 1, name: juno, loglevel: info, startmode: 1}
  - {gid: 4, name: dpv2, loglevel: info, startmode: 1}
""")
        (root / "fatima-package-predefine.properties").write_text(f"var.global.package.groupname=linux-deploy\nvar.global.package.hostname=linux{i:02}\nvar.global.package.name=default\nvar.host.ipaddress=${{var.builtin.local.ipaddress}}\nvar.saturn.enable=false\n")
        program = "jupiter" if i == 0 else "juno"
        props = "repo=file\nwebserver.address=0.0.0.0\nwebserver.port=9190\n" if i == 0 else "gateway.address=fatima-v2-jupiter\ngateway.port=9190\nwebserver.port=9180\nwebserver.address=${var.builtin.local.ipaddress}\n"
        (root / "application.properties").write_text(props)
        command(["docker", "run", "-d", "--init", "--name", name, "--network", network, "--label", "fatima.test=progressive-deploy", "-v", f"{root / 'fatima-package.yaml'}:/fatima/conf/fatima-package.yaml:ro", "-v", f"{root / 'fatima-package-predefine.properties'}:/fatima/conf/fatima-package-predefine.properties:ro", "-v", f"{root / 'application.properties'}:/fatima/app/{program}/application.properties:ro", "fatima-v2-local", f"/fatima/app/{program}/{program}"])
        if i == 0:
            time.sleep(1)
    for _ in range(30):
        output = inside(names[0], ["ropack"])
        if "package:3" in output:
            break
        time.sleep(0.5)
    else:
        raise RuntimeError("three Juno registrations missing: " + output)
    print("three Linux Juno packages registered", flush=True)
    subprocess.run(["docker", "cp", str(lab / "linux/dpv2.far"), names[0] + ":/tmp/dpv2.far"], check=True)
    a = cli("--request-id", "linux-upload-once", "upload", "/tmp/dpv2.far")
    print("uploaded", a["id"], a["sha256"], flush=True)
    for number in (1, 2):
        p = cli("--request-id", f"linux-rollout-{number}", "--artifact", a["id"], "-g", "linux-deploy", "-p", "linux01:default", "create")
        p = wait(p["id"], "WAITING")
        assert all(t["operation"]["state"] == "QUEUED" for t in p["targets"][1:])
        print("first package completed, remaining packages stayed queued", flush=True)
        p = cli("--action", "continue", "--revision", p["revision"], "act", p["id"])
        p = wait(p["id"], "SUCCEEDED")
        assert all(t["operation"]["sha256"] == a["sha256"] for t in p["targets"])
        (lab / f"linux-result-{number}.json").write_text(json.dumps(p, indent=2))
        if number == 2:
            for t in p["targets"]:
                assert any(e["stage"] == "goaway" and e.get("message") == "goaway completed" for e in t["operation"]["events"])
        print("rollout", number, "completed on all three packages", flush=True)
    assert len(cli("artifacts")["artifacts"]) == 1
    print(inside(names[0], ["rodis", "-p", "linux01:default"]), flush=True)
    print("PASS: real Linux lifecycle, native gRPC, legacy HTTP, one upload reused for all targets", flush=True)
finally:
    for name in names:
        result = subprocess.run(["docker", "logs", name], capture_output=True, text=True)
        (lab / (name + ".log")).write_text(result.stdout + result.stderr)
        subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["docker", "network", "rm", network], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
