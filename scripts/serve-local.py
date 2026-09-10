#!/usr/bin/env python3
"""Keep the local OPM test processes attached to a test supervisor."""
import json
import os
from pathlib import Path
import signal
import subprocess
import time

base = Path(__file__).resolve().parents[1]
lab = base / ".local/progressive-deploy"
home = Path(os.environ["FATIMA_HOME"])
children = []
try:
    for name in ("jupiter", "juno", "saturn"):
        output = (lab / (name + ".console.log")).open("ab", buffering=0)
        p = subprocess.Popen([str(home / "app" / name / name)], cwd=home / "app" / name, stdout=output, stderr=output, start_new_session=True)
        children.append(p)
        print(name, p.pid, flush=True)
        time.sleep(1)
    (lab / "supervisor.json").write_text(json.dumps({"pid": os.getpid(), "children": [p.pid for p in children]}))
    while all(p.poll() is None for p in children):
        time.sleep(1)
except KeyboardInterrupt:
    pass
finally:
    for p in children:
        if p.poll() is None:
            p.send_signal(signal.SIGTERM)
    for p in children:
        try:
            p.wait(timeout=10)
        except subprocess.TimeoutExpired:
            p.kill()
    print("local OPM supervisor stopped", flush=True)
