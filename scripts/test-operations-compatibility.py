#!/usr/bin/env python3
"""Exercise six operating CLIs against archived/current OPM binaries on the local Mac.

Requires the progressive-deploy local fixture (local01:default, ctlprobe) and
the archived original Darwin package. Replaces local OPM binaries temporarily;
always restores the installed binaries in finally. Never contacts GitHub.
"""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import time
import urllib.request

base = Path(__file__).resolve().parents[1]
lab = base / ".local/progressive-deploy"
home = Path(os.environ["FATIMA_HOME"])
work = lab / "operations-compatibility"
commands = "rocron rostop rostart rodis ropack roproc".split()
target = "local01:default"


def call(args, timeout=90, detached=False):
    return subprocess.check_output([str(x) for x in args], text=True, stderr=subprocess.STDOUT, timeout=timeout, start_new_session=detached)


def guard():
    context = call([home / "bin/rocontext"])
    if not re.search(r"^\*\s+local\s+http://127\.0\.0\.1:9190\s", context, re.M):
        raise RuntimeError("the active rocontext must be local at 127.0.0.1:9190")


def alive(name):
    path = home / "app" / name / "proc" / (name + ".pid")
    if not path.exists() or not path.read_text().strip().isdigit():
        return False
    pid = path.read_text().strip()
    result = subprocess.run(["ps", "-p", pid, "-o", "stat="], capture_output=True, text=True)
    return result.returncode == 0 and bool(result.stdout.strip()) and not result.stdout.strip().startswith("Z")


def await_state(name, state, seconds=25):
    until = time.monotonic() + seconds
    while alive(name) != state:
        if time.monotonic() >= until:
            raise RuntimeError(f"{name}: expected alive={state}")
        time.sleep(.2)


def switch(gateway, backend):
    guard()
    call([home / "bin/stopro", "-y"])
    for name in ("jupiter", "juno", "saturn"):
        await_state(name, False, 45)
    for name, generation in (("jupiter", gateway), ("juno", backend)):
        dest = home / "app" / name / name
        pending = dest.with_name(name + ".compat-update")
        shutil.copy2(work / generation / name, pending)
        pending.replace(dest)
    call([home / "bin/startro", "-y"], detached=True)
    for name in ("jupiter", "juno", "saturn"):
        await_state(name, True)
    until = time.monotonic() + 20
    while True:
        output = call([work / "old/ropack"])
        if "local01" in output:
            break
        if time.monotonic() > until:
            raise RuntimeError("local Juno did not register")
        time.sleep(.5)
    if gateway == "new":
        with urllib.request.urlopen("http://127.0.0.1:9190/.well-known/fatima/capabilities", timeout=3) as response:
            assert "roproc" in json.load(response)["features"]


def exercise(label, client):
    output = []

    def cli(name, *args):
        guard()
        value = call([work / client / name, *args])
        output.append("$ " + name + " " + " ".join(args) + "\n" + value)
        (work / (label + ".log")).write_text("\n".join(output))
        return value

    assert "local01" in cli("ropack")
    assert "ctlprobe" in cli("rodis", "-p", target)
    assert "control.probe" in cli("rocron", "-p", target, "-l")
    cli("rostop", "-p", target, "ctlprobe")
    await_state("ctlprobe", False)
    cli("rostart", "-p", target, "ctlprobe")
    await_state("ctlprobe", True)
    if "ctlcompat" in (home / "conf/fatima-package.yaml").read_text():
        raise RuntimeError("ctlcompat must not already be registered")
    cli("roproc", "-p", target, "add", "ctlcompat", "4")
    assert "ctlcompat" in (home / "conf/fatima-package.yaml").read_text()
    cli("roproc", "-p", target, "remove", "ctlcompat")
    assert "ctlcompat" not in (home / "conf/fatima-package.yaml").read_text()
    print("PASS", label, "(six commands)", flush=True)


def main():
    if os.uname().sysname != "Darwin" or not home.is_absolute():
        raise RuntimeError("this fixture is for a configured local Mac")
    guard()
    for generation in ("new", "old"):
        (work / generation).mkdir(parents=True, exist_ok=True)
    for name in commands:
        shutil.copy2(home / "bin" / name, work / "new" / name)
    for name in ("jupiter", "juno"):
        shutil.copy2(home / "app" / name / name, work / "new" / name)
    with tarfile.open(lab / "legacy/fatima-package.darwin-arm64.tar.gz") as archive:
        for name in commands + ["jupiter", "juno"]:
            member = "fatima-package/" + ("app/" + name + "/" + name if name in ("jupiter", "juno") else "bin/" + name)
            (work / "old" / name).write_bytes(archive.extractfile(member).read())
            (work / "old" / name).chmod(0o755)
    passed = []
    try:
        for gateway, backend, client in (("new", "new", "old"), ("old", "old", "new"), ("new", "old", "new"), ("old", "new", "new")):
            label = f"jupiter-{gateway}_juno-{backend}_cli-{client}"
            switch(gateway, backend)
            exercise(label, client)
            passed.append(label)
    finally:
        switch("new", "new")
        await_state("ctlprobe", True)
        (work / "result.json").write_text(json.dumps({"passed": passed, "restored": "new/new", "commands": commands}, indent=2) + "\n")


if __name__ == "__main__":
    main()
