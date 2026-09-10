#!/usr/bin/env python3
"""Apply a local package archive with updater semantics, without GitHub."""
import argparse
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import tempfile
import time


def alive(pid):
    status = subprocess.run(["ps", "-p", str(pid), "-o", "stat="], capture_output=True, text=True)
    return status.returncode == 0 and status.stdout.strip() and not status.stdout.strip().startswith("Z")


def install():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("archive", type=Path)
    p.add_argument("--home", type=Path, default=Path(os.environ["FATIMA_HOME"]) if os.environ.get("FATIMA_HOME") else None)
    p.add_argument("--initialize", action="store_true", help="also copy missing configuration/resource files")
    p.add_argument("--restart", action="store_true", help="stop and restart the locally configured OPM processes")
    opts = p.parse_args()
    if opts.home is None or not opts.home.is_absolute() or opts.home == Path("/"):
        p.error("an absolute FATIMA_HOME / --home is required")
    env = dict(os.environ, FATIMA_HOME=str(opts.home), PATH=str(opts.home / "bin") + os.pathsep + os.environ.get("PATH", "") + os.pathsep + ".")
    with tempfile.TemporaryDirectory(prefix="fatima-install-") as directory:
        with tarfile.open(opts.archive) as tar:
            for member in tar.getmembers():
                path = Path(member.name)
                if path.is_absolute() or ".." in path.parts or path.parts[0] != "fatima-package" or not (member.isfile() or member.isdir()):
                    raise ValueError("unsafe package entry: " + member.name)
            # macOS system Python can predate tarfile's filter parameter. All
            # entries were validated above and links/special files rejected.
            tar.extractall(directory)
        package = Path(directory) / "fatima-package"
        info = json.loads((package / "packing-info.json").read_text())
        arch = {"aarch64": "arm64", "x86_64": "amd64"}.get(platform.machine(), platform.machine())
        if info.get("os") != platform.system().lower() or info.get("architecture") != arch:
            raise ValueError("package platform does not match this machine")
        if opts.restart and (opts.home / "bin/stopro").exists():
            pids = []
            for name in ("jupiter", "juno", "saturn"):
                pid_file = opts.home / "app" / name / "proc" / (name + ".pid")
                if pid_file.exists() and pid_file.read_text().strip().isdigit():
                    pids.append(int(pid_file.read_text().strip()))
            subprocess.run([str(opts.home / "bin/stopro"), "-y"], env=env, check=True)
            until = time.monotonic() + 40
            while any(alive(pid) for pid in pids):
                if time.monotonic() >= until:
                    raise RuntimeError("OPM shutdown timed out; package files were not replaced")
                time.sleep(0.2)
        opts.home.mkdir(parents=True, exist_ok=True)
        if opts.initialize:
            for src in package.rglob("*"):
                dst = opts.home / src.relative_to(package)
                if src.is_dir():
                    dst.mkdir(parents=True, exist_ok=True)
                elif not dst.exists():
                    shutil.copy2(src, dst)
        (opts.home / "bin").mkdir(exist_ok=True)
        paths = list((package / "bin").iterdir()) + [package / "packing-info.json"]
        # Like roupdate all, update existing OPM installations; do not overwrite
        # their application.properties or package registration configuration.
        for name in ("jupiter", "juno", "saturn"):
            if (opts.home / "app" / name).is_dir():
                paths.append(package / "app" / name / name)
        for src in paths:
            dst = opts.home / src.relative_to(package)
            temp = dst.with_name(dst.name + ".local-update")
            shutil.copy2(src, temp)
            temp.replace(dst)
        if opts.restart:
            subprocess.run([str(opts.home / "bin/startro"), "-y"], env=env, check=True, start_new_session=True)
    print("installed", opts.archive, "into", opts.home)


if __name__ == "__main__":
    install()
