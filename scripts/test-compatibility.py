#!/usr/bin/env python3
"""Run archived legacy binaries against both server generations in Linux."""
import json
import fcntl
import os
from pathlib import Path
import pty
import select
import struct
import subprocess
import termios
import time

base = Path(__file__).resolve().parents[1]
lab = base / ".local/progressive-deploy"
network = "fatima-v2-compat"
gateway = "fatima-v2-jupiter"
node = "fatima-v2-compat-juno"


def call(args):
    return subprocess.check_output(args, text=True).strip()


def inside(name, args):
    return call(["docker", "exec", name, *args])


def terminal(args, steps):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 38, 120, 0, 0))
    process = subprocess.Popen(args, stdin=slave, stdout=slave, stderr=slave, close_fds=True)
    os.close(slave)
    output = ""
    pending = ""
    try:
        for expected, keys in steps:
            until = time.monotonic() + 30
            while expected not in pending:
                if time.monotonic() > until:
                    raise RuntimeError("TUI did not show " + expected + ": " + output[-1500:])
                if select.select([master], [], [], 0.5)[0]:
                    chunk = os.read(master, 65536).decode(errors="replace")
                    output += chunk
                    pending += chunk
            os.write(master, keys)
            pending = ""
        process.wait(timeout=10)
    finally:
        if process.poll() is None:
            process.kill()
        os.close(master)
    return output


cases = [(False, False, "old"), (True, True, "old"), (False, True, "new"), (True, False, "tui")]
for number, (new_gateway, new_juno, client) in enumerate(cases):
    name = f"gateway-{'new' if new_gateway else 'old'}_juno-{'new' if new_juno else 'old'}_cli-{client}"
    try:
        call(["docker", "network", "create", network])
        for container, program, new in ((gateway, "jupiter", new_gateway), (node, "juno", new_juno)):
            root = lab / "linux" / ("matrix-" + name + "-" + program)
            root.mkdir(exist_ok=True)
            src = lab / "linux" / ("fatima-v2-jupiter" if program == "jupiter" else "fatima-v2-juno1")
            for file in ("fatima-package.yaml", "fatima-package-predefine.properties", "application.properties"):
                (root / file).write_bytes((src / file).read_bytes())
            args = ["docker", "run", "-d", "--init", "--name", container, "--network", network, "--label", "fatima.test=progressive-deploy"]
            for file, dest in (("fatima-package.yaml", "/fatima/conf/fatima-package.yaml"), ("fatima-package-predefine.properties", "/fatima/conf/fatima-package-predefine.properties"), ("application.properties", f"/fatima/app/{program}/application.properties")):
                args += ["-v", f"{root / file}:{dest}:ro"]
            if not new:
                args += ["-v", f"{lab / 'linux/legacy' / program}:/fatima/app/{program}/{program}:ro"]
            args += ["fatima-v2-local", f"/fatima/app/{program}/{program}"]
            call(args)
            if program == "jupiter":
                time.sleep(1)
        for _ in range(30):
            if "package:1" in inside(gateway, ["ropack"]):
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("Juno did not register")
        for source, dest in ((lab / "gopath/far/dpv2/dpv2.far", "/tmp/dpv2.far"), (lab / "linux/legacy/rodeploy", "/tmp/old-rodeploy"), (lab / "linux/rodeploy", "/tmp/new-rodeploy")):
            subprocess.run(["docker", "cp", str(source), gateway + ":" + dest], check=True)
        if client == "tui":
            # The new Jupiter advertises v2 while the target Juno explicitly
            # lacks it. The TUI must choose legacy before creating a rollout.
            output = terminal(["docker", "exec", "-it", gateway, "/tmp/new-rodeploy", "-g", "linux-deploy", "upload", "/tmp/dpv2.far"], [("FAR 확인 완료", b"\r"), ("artifacts loaded", b"\r"), ("Choose the first package", b"\r"), ("FAR 확인 완료", b"\r"), ("CONFIRM · legacy", b"\r"), ("배포 요청 완료", b"\x03")])
            plans = json.loads(inside(gateway, ["/tmp/new-rodeploy", "--json", "rollouts"]))
            assert not plans.get("rollouts")
        else:
            output = inside(gateway, ["/tmp/" + client + "-rodeploy", "-g", "linux-deploy", "/tmp/dpv2.far"])
            assert "enqueued" in output, output
        (lab / ("compatibility-" + name + ".log")).write_text(output)
        for _ in range(40):
            result = subprocess.run(["docker", "exec", node, "sh", "-c", "test -f /fatima/app/dpv2/proc/dpv2.pid && kill -0 \"$(cat /fatima/app/dpv2/proc/dpv2.pid)\""], capture_output=True)
            if result.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("legacy deployment did not actually start the process")
        print("PASS", name, flush=True)
    finally:
        for container in (gateway, node):
            subprocess.run(["docker", "cp", container + ":/fatima/log", str(lab / "linux" / ("logs-" + name + "-" + container))], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(["docker", "network", "rm", network], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
