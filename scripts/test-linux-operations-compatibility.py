#!/usr/bin/env python3
"""Mac old/new operating CLIs against old/new Linux arm64 Jupiter and Juno."""
import importlib.util
import json
from pathlib import Path
import subprocess
import time

spec = importlib.util.spec_from_file_location("checks", Path(__file__).with_name("test-linux-operations.py"))
checks = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checks)
lab = checks.lab
OUT = lab.LAB / "compatibility"
LEGACY = lab.BASE / ".local/progressive-deploy/legacy"
OLD_LINUX = lab.BASE / ".local/progressive-deploy/linux/legacy"


def command(name, *args, old=False):
    lab.local_guard()
    binary = (LEGACY if old else lab.MAC_HOME / "bin") / name
    result = subprocess.run([str(binary), *args], capture_output=True, text=True, timeout=160)
    with (OUT / "commands.log").open("a") as f:
        f.write(f"$ {'old' if old else 'new'} {name} {' '.join(args)}\n" + result.stdout + result.stderr + f"\nexit={result.returncode}\n")
    if result.returncode != 0:
        raise RuntimeError(name + ": " + result.stdout + result.stderr)
    return result.stdout


def switch(gateway, backend):
    lab.local_guard()
    lab.inside("stopro", "-y")
    checks.eventually(lambda: all(not checks.live(p) for p in ("jupiter", "juno", "saturn")), 45)
    for name, generation in (("jupiter", gateway), ("juno", backend)):
        lab.inside("cp", "/tmp/compat/" + generation + "/" + name, "/fatima/app/" + name + "/" + name)
    lab.inside("startro", "-y")
    checks.eventually(lambda: all(checks.live(p) for p in ("jupiter", "juno", "saturn")))
    checks.eventually(lambda: "linux01" in command("ropack", old=True))


def exercise(old):
    assert "linux01" in command("ropack", old=old)
    assert "linux_arm64" in command("rodis", "-p", lab.TARGET, old=old)
    assert "control.probe" in command("rocron", "-p", lab.TARGET, "-l", old=old)
    command("rostop", "-p", lab.TARGET, "ctlprobe", old=old)
    checks.eventually(lambda: not checks.live("ctlprobe"), 60)
    command("rostart", "-p", lab.TARGET, "ctlprobe", old=old)
    checks.eventually(lambda: checks.live("ctlprobe"))
    assert "ctlcompat" not in checks.contents("/fatima/conf/fatima-package.yaml")
    command("roproc", "-p", lab.TARGET, "add", "ctlcompat", "4", old=old)
    assert "ctlcompat" in checks.contents("/fatima/conf/fatima-package.yaml")
    command("roproc", "-p", lab.TARGET, "remove", "ctlcompat", old=old)
    checks.eventually(lambda: "ctlcompat" not in checks.contents("/fatima/conf/fatima-package.yaml"))


def main():
    OUT.mkdir(exist_ok=True)
    lab.local_guard()
    for name in ("jupiter", "juno"):
        b = (OLD_LINUX / name).read_bytes()[:20]
        assert b[:4] == b"\x7fELF" and int.from_bytes(b[18:20], "little") == 183
        lab.inside("mkdir", "-p", "/tmp/compat/new", "/tmp/compat/old")
        lab.inside("cp", "/fatima/app/" + name + "/" + name, "/tmp/compat/new/" + name)
        lab.docker("cp", str(OLD_LINUX / name), lab.NAME + ":/tmp/compat/old/" + name)
    passed = []
    try:
        for gateway, backend, old_client in (("new", "new", True), ("old", "old", False), ("new", "old", False), ("old", "new", False)):
            label = f"jupiter-{gateway}_juno-{backend}_mac-cli-{'old' if old_client else 'new'}"
            switch(gateway, backend)
            exercise(old_client)
            passed.append(label)
            print("PASS", label, "(all six operating commands)", flush=True)
    finally:
        switch("new", "new")
        lab.wait_ready()
        (OUT / "result.json").write_text(json.dumps({"passed": passed, "restored": "new/new"}, indent=2) + "\n")


if __name__ == "__main__":
    main()
