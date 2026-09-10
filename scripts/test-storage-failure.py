#!/usr/bin/env python3
"""After test-linux.py, verify failed v2 storage cannot disable legacy HTTP."""
import json
from pathlib import Path
import subprocess
import time

base = Path(__file__).resolve().parents[1]
lab = base / ".local/progressive-deploy"
network = "fatima-v2-storage-test"
gateway, node = "fatima-v2-jupiter", "fatima-v2-storage-juno"


def call(args):
    return subprocess.check_output(args, text=True).strip()


def inside(name, args):
    return call(["docker", "exec", name, *args])


for failing in ("jupiter", "juno"):
    try:
        call(["docker", "network", "create", network])
        for name, program in ((gateway, "jupiter"), (node, "juno")):
            root = lab / "linux" / ("storage-failure-" + failing + "-" + program)
            root.mkdir(exist_ok=True)
            source = lab / "linux" / ("fatima-v2-jupiter" if program == "jupiter" else "fatima-v2-juno1")
            for file in ("fatima-package.yaml", "fatima-package-predefine.properties", "application.properties"):
                value = (source / file).read_text()
                if file == "application.properties" and program == failing:
                    value += "deployment.v2.storage=/dev/null/blocked\n"
                (root / file).write_text(value)
            args = ["docker", "run", "-d", "--init", "--name", name, "--network", network, "--label", "fatima.test=progressive-deploy"]
            for file, dest in (("fatima-package.yaml", "/fatima/conf/fatima-package.yaml"), ("fatima-package-predefine.properties", "/fatima/conf/fatima-package-predefine.properties"), ("application.properties", f"/fatima/app/{program}/application.properties")):
                args += ["-v", f"{root / file}:{dest}:ro"]
            call(args + ["fatima-v2-local", f"/fatima/app/{program}/{program}"])
            if program == "jupiter":
                time.sleep(1)
        for _ in range(30):
            if "package:1" in inside(gateway, ["ropack", "--legacy"]):
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("legacy Juno registration failed")
        assert "linux01" in inside(gateway, ["rodis", "--legacy", "-p", "linux01:default"])
        endpoint = f"http://{gateway}:9190" if failing == "jupiter" else f"http://{node}:9180"
        caps = json.loads(inside(gateway, ["wget", "-qO-", endpoint + "/.well-known/fatima/capabilities"]))
        assert "unavailable" in caps["features"], caps
        if failing == "jupiter":
            result = subprocess.run(["docker", "exec", gateway, "rodeploy", "--json", "artifacts"], capture_output=True, text=True)
            assert result.returncode != 0 and "does not advertise" in result.stderr, result
        else:
            subprocess.run(["docker", "cp", str(lab / "linux/dpv2.far"), gateway + ":/tmp/dpv2.far"], check=True)
            artifact = json.loads(inside(gateway, ["rodeploy", "--json", "--request-id", "upload", "upload", "/tmp/dpv2.far"]))
            result = subprocess.run(["docker", "exec", gateway, "rodeploy", "--json", "--request-id", "blocked", "--artifact", artifact["id"], "-g", "linux-deploy", "-p", "linux01:default", "create"], capture_output=True, text=True)
            assert result.returncode != 0 and "Juno v2 unavailable" in result.stderr, result
            assert not json.loads(inside(gateway, ["rodeploy", "--json", "rollouts"])).get("rollouts")
        (lab / ("storage-failure-" + failing + ".log")).write_text(result.stderr)
        print("PASS:", failing, "storage unavailable; legacy HTTP active; new deployment blocked", flush=True)
    finally:
        for name in (gateway, node):
            subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(["docker", "network", "rm", network], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
