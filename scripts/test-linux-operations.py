#!/usr/bin/env python3
"""Actual Mac CLI -> Linux OPM regression tests; leave the current Linux lab running."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import time
import uuid

spec = importlib.util.spec_from_file_location("linux_local", Path(__file__).with_name("linux-local.py"))
lab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lab)
OUT = lab.LAB / "checks"
RUN = "linux-" + uuid.uuid4().hex[:10]
passed = []


def cli(name, *args, ok=True, binary=None):
    lab.local_guard()
    command = [str(binary or lab.MAC_HOME / "bin" / name), *args]
    result = subprocess.run(command, capture_output=True, text=True, timeout=160)
    with (OUT / "commands.log").open("a") as f:
        f.write("$ " + " ".join(command) + "\n" + result.stdout + result.stderr + f"\nexit={result.returncode}\n")
    if ok and result.returncode != 0:
        raise RuntimeError(name + " failed: " + result.stdout + result.stderr)
    if not ok and result.returncode == 0:
        raise RuntimeError(name + " unexpectedly succeeded: " + result.stdout)
    return result


def query(name, *args, ok=True):
    return json.loads(cli(name, "--json", "-p", lab.TARGET, *args, ok=ok).stdout)


def check(label):
    passed.append(label)
    print("PASS", label, flush=True)
    (OUT / "result.json").write_text(json.dumps({"run": RUN, "passed": passed}, indent=2) + "\n")


def contents(path):
    return lab.inside("cat", path)


def live(name):
    # Compare the API to the real Linux process, including zombie exclusion.
    script = 'p=$(cat "/fatima/app/$1/proc/$1.pid" 2>/dev/null) || exit 1; test "$p" -gt 1 || exit 1; test -r "/proc/$p/stat" || exit 1; read -r pid comm state rest < "/proc/$p/stat"; test "$state" != Z || exit 1; printf "%s" "$p"'
    result = subprocess.run(["docker", "exec", lab.NAME, "sh", "-c", script, "sh", name], env=lab.DOCKER_ENV, capture_output=True, text=True)
    return result.stdout.strip() if result.returncode == 0 else None


def eventually(fn, seconds=30):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        result = fn()
        if result:
            return result
        time.sleep(.3)
    raise RuntimeError("condition did not become true")


def operation(name, process, suffix, expected="SUCCEEDED"):
    result = query(name, "--request-id", RUN + "-" + suffix, process, ok=expected != "FAILED")
    assert result["state"] == expected, result
    return result


def restart(name, signal="TERM"):
    pid = live(name)
    if pid:
        lab.inside("kill", "-" + signal, pid)
        eventually(lambda: not live(name))
    lab.docker("exec", "-d", "-w", "/fatima/app/" + name, lab.NAME, "/fatima/app/" + name + "/" + name)
    eventually(lambda: live(name))
    lab.wait_ready()


def copy_probe(name, fail=False):
    lab.inside("mkdir", "-p", "/fatima/app/" + name)
    if fail:
        lab.inside("sh", "-c", 'printf "#!/bin/sh\\nexit 7\\n" > /fatima/app/ctlfail/ctlfail; chmod +x /fatima/app/ctlfail/ctlfail')
    else:
        lab.inside("cp", "/fatima/app/ctlprobe/ctlprobe", "/fatima/app/" + name + "/" + name)
        lab.inside("cp", "/fatima/app/ctlprobe/application.properties", "/fatima/app/" + name + "/application.properties")


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    lab.local_guard()
    assert "aarch64" in lab.inside("uname", "-m")
    assert "linux_arm64" in cli("rodis", "-p", lab.TARGET).stdout
    initial = query("roproc")["catalog"]
    assert initial["platform"] == "linux_arm64"
    assert all(live(p) for p in ("jupiter", "juno", "saturn"))
    packages = json.loads(cli("ropack", "--json").stdout)["packages"]
    package = next(p for p in packages if p["target"]["package_id"] == lab.TARGET)
    assert package["state"] == "ALIVE" and package["transport"] == "gRPC"
    check("Mac -> Linux arm64; shared 9190/9180; ropack and rodis")

    operation("rostart", "ctlprobe", "initial-start")
    pid = live("ctlprobe")
    assert pid
    operation("rostart", "ctlprobe", "already-started")
    assert live("ctlprobe") == pid
    catalog = eventually(lambda: query("rocron").get("jobs"))
    assert any(j["name"] == "control.probe" for j in catalog)
    args = ["--process", "ctlprobe", "--job", "control.probe", "--arguments", RUN, "--request-id", RUN + "-cron"]
    first = query("rocron", *args)
    assert first["state"] == "REQUESTED"
    duplicate = query("rocron", *args)
    assert first == duplicate
    eventually(lambda: RUN in contents("/fatima/data/ctlprobe/requests.log"))
    time.sleep(1)
    assert contents("/fatima/data/ctlprobe/requests.log").splitlines().count(RUN) == 1
    conflict = args.copy()
    conflict[5] = RUN + "-different"
    assert "AlreadyExists" in cli("rocron", "--json", "-p", lab.TARGET, *conflict, ok=False).stderr
    assert query("rocron", "--watch", RUN + "-cron") == first
    check("rocron actual IPC delivery, one execution per ID, conflict and reconnect")

    stopped = operation("rostop", "ctlprobe", "stop")
    assert not live("ctlprobe")
    assert any(e["stage"].endswith("/goaway") for e in stopped["events"])
    assert any(e["stage"].endswith("/shutdown") and "exited" in e["message"] for e in stopped["events"])
    assert operation("rostop", "ctlprobe", "stop") == stopped
    # Linux metrics are sampled asynchronously by the existing monitor. The
    # operation above already proved actual PID exit; await its next snapshot.
    eventually(lambda: next(p for p in query("roproc")["catalog"]["processes"] if p["name"] == "ctlprobe")["state"] == "DEAD")
    started = operation("rostart", "ctlprobe", "start")
    assert live("ctlprobe") and live("ctlprobe") != pid
    assert "readiness" in started["events"][-2]["message"] or any("readiness" in e["message"] for e in started["events"])
    for _ in range(20):
        report = query("roproc")["catalog"]
        by_name = {p["name"]: p for p in report["processes"]}
        assert len(by_name) == len(report["processes"]) == 5
        for name in ("jupiter", "juno", "saturn", "ctlprobe"):
            assert by_name[name]["state"] == "ALIVE" and by_name[name]["pid"] == live(name), report
    check("rostop drain and actual PID exit; rostart survival; 20 complete status snapshots")

    # Writable registration, program lifecycle, and all removal paths.
    # Legacy roproc deliberately retains its original YAML rewrite behavior;
    # restore this fixture comment if a compatibility run removed it earlier.
    if "Registry changes must preserve this comment" not in contents("/fatima/conf/fatima-package.yaml"):
        lab.inside("sh", "-c", "printf '\\n# Registry changes must preserve this comment.\\n' >> /fatima/conf/fatima-package.yaml")
    copy_probe("ctlregprobe")
    added = query("roproc", "--request-id", RUN + "-add", "add", "ctlregprobe", "4")
    assert added["state"] == "SUCCEEDED"
    operation("rostart", "ctlregprobe", "registered-start")
    assert live("ctlregprobe")
    lab.inside("mkdir", "-p", "/fatima/app/revision/ctlregprobe/test", "/fatima/log/ctlregprobe", "/fatima/data/ctlregprobe")
    removed = query("roproc", "--request-id", RUN + "-remove", "remove", "ctlregprobe")
    assert removed["state"] == "SUCCEEDED" and not live("ctlregprobe")
    for path in ("app/ctlregprobe", "app/revision/ctlregprobe", "log/ctlregprobe", "data/ctlregprobe"):
        lab.inside("test", "!", "-e", "/fatima/" + path)
    assert "ctlregprobe" not in contents("/fatima/conf/fatima-package.yaml")
    assert "Registry changes must preserve this comment" in contents("/fatima/conf/fatima-package.yaml")
    assert query("roproc", "--watch", RUN + "-remove")["state"] == "SUCCEEDED"
    cli("roproc", "--json", "-p", lab.TARGET, "remove", "juno", ok=False)
    check("roproc add/start/drain/remove, filesystem cleanup, YAML comments and OPM protection")

    copy_probe("ctlfail", fail=True)
    query("roproc", "--request-id", RUN + "-fail-add", "add", "ctlfail", "4")
    failed = operation("rostart", "ctlfail", "failed-start", "FAILED")
    assert not live("ctlfail")
    assert query("rostart", "--watch", RUN + "-failed-start", ok=False)["state"] == "FAILED"
    query("roproc", "--request-id", RUN + "-fail-remove", "remove", "ctlfail")
    check("immediate startup failure, nonzero CLI exit and saved failed result")

    # The client disappears after submission; the server must finish only once.
    request_id = RUN + "-disconnect"
    process = subprocess.Popen([str(lab.MAC_HOME / "bin/rostop"), "--json", "-p", lab.TARGET, "--request-id", request_id, "ctlprobe"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        eventually(lambda: request_id in contents("/fatima/data/juno/control-v2/requests.json"))
        process.kill()
        process.communicate(timeout=10)
        eventually(lambda: not live("ctlprobe"))
        eventually(lambda: query("rostop", "--watch", request_id)["state"] == "SUCCEEDED")
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate()
    operation("rostart", "ctlprobe", "after-disconnect")
    check("client disconnect does not cancel or duplicate server operation")

    # Abrupt server death while a real operation is active must not replay it.
    request_id = RUN + "-interrupted"
    process = subprocess.Popen([str(lab.MAC_HOME / "bin/rostop"), "--json", "-p", lab.TARGET, "--request-id", request_id, "ctlprobe"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    try:
        eventually(lambda: request_id in contents("/fatima/data/juno/control-v2/requests.json"))
        restart("juno", "KILL")
        process.communicate(timeout=15)
        result = query("rostop", "--watch", request_id, ok=False)
        assert result["state"] == "INTERRUPTED", result
        assert query("rocron", "--watch", RUN + "-cron")["state"] == "REQUESTED"
        operation("rostart", "ctlprobe", "after-restart")
    finally:
        if process.poll() is None:
            process.kill()
            process.communicate()
    check("Juno restart preserves results, marks interrupted operation and does not replay")

    # Execute the shipped Linux CLI as well as the native Mac CLI.
    for name, args in (("ropack", []), ("rocron", ["-p", lab.TARGET]), ("roproc", ["-p", lab.TARGET])):
        json.loads(lab.inside(name, "--json", *args))
    json.loads(lab.inside("rostop", "--json", "-p", lab.TARGET, "--request-id", RUN + "-linux-stop", "ctlprobe"))
    json.loads(lab.inside("rostart", "--json", "-p", lab.TARGET, "--request-id", RUN + "-linux-start", "ctlprobe"))
    assert "linux_arm64" in lab.inside("rodis", "-p", lab.TARGET)
    assert live("ctlprobe")
    check("all six shipped Linux arm64 CLI binaries run against real Linux OPM")


if __name__ == "__main__":
    main()
