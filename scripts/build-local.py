#!/usr/bin/env python3
"""Build the sibling working trees; never clone, push, or fetch source repos."""
import argparse
import getpass
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
from datetime import datetime

PACKAGE = Path(__file__).resolve().parents[1]
SOURCES = ("fatima-core", "fatima-log", "fatima-cmd", "jupiter", "juno", "saturn", "gofar")
COMMANDS = "lcslack lcproc lccrypto rocontext roupdate roclip rocron rodeploy roclric rohis rodis rolog ropack roproc lcps rostart rostop lcha startro stopro".split()


def run(args, cwd=None, env=None):
    return subprocess.check_output(args, cwd=cwd, env=env, text=True).strip()


def build():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--os", choices=("darwin", "linux"), default="darwin")
    p.add_argument("--arch", choices=("arm64", "amd64"), default="arm64")
    p.add_argument("--source-root", type=Path, default=PACKAGE.parent)
    p.add_argument("--output", type=Path, default=PACKAGE.parent / "fatima-download")
    opts = p.parse_args()
    opts.output.mkdir(parents=True, exist_ok=True)
    scratch = PACKAGE / ".local"
    scratch.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="build-", dir=scratch) as directory:
        work = Path(directory)
        identities = {}
        for name in SOURCES:
            source = opts.source_root / name
            target = work / name
            target.mkdir()
            names = subprocess.check_output(["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"], cwd=source).decode().split("\0")
            digest = hashlib.sha256()
            for relative in sorted(set(names)):
                if not relative:
                    continue
                src = source / relative
                if not src.is_file():
                    continue
                digest.update(relative.encode() + b"\0" + src.read_bytes())
                dst = target / relative
                dst.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src, dst)
            # Some historical repos ignore their dependency lock file.
            if (source / "go.sum").exists():
                shutil.copy2(source / "go.sum", target / "go.sum")
            identities[name] = {"commit": run(["git", "rev-parse", "HEAD"], source), "branch": run(["git", "branch", "--show-current"], source), "dirty": bool(run(["git", "status", "--porcelain"], source)), "source_sha256": digest.hexdigest()}
        gowork = work / "go.work"
        version = run(["go", "env", "GOVERSION"]).removeprefix("go")
        gowork.write_text("go " + version + "\n\nuse (\n" + "".join(f"  ./{name}\n" for name in SOURCES) + ")\n")
        env = dict(os.environ, GOWORK=str(gowork), GOOS=opts.os, GOARCH=opts.arch, CGO_ENABLED="0")
        package = work / "fatima-package"
        shutil.copytree(PACKAGE / "resources/standard", package)
        (package / "bin").mkdir()
        for command in COMMANDS:
            print("building", command, flush=True)
            subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(package / "bin" / command), "./cmd/" + command], cwd=work / "fatima-cmd", env=env, check=True)
        print("building gofar", flush=True)
        subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(package / "bin/gofar"), "."], cwd=work / "gofar", env=env, check=True)
        for name in ("jupiter", "juno", "saturn"):
            print("building", name, flush=True)
            subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(package / "app" / name / name), "."], cwd=work / name, env=env, check=True)
        # Keep the fields and display format consumed by the existing updater.
        info = {"user": getpass.getuser(), "build_time": datetime.now().astimezone().strftime("%Y-%m-%d %H:%M:%S %Z"), "os": opts.os, "architecture": opts.arch, "source": "local-working-trees", "repositories": identities}
        (package / "packing-info.json").write_text(json.dumps(info, indent=2) + "\n")
        name = f"fatima-package.{opts.os}-{opts.arch}.tar.gz"
        pending = opts.output / (name + ".tmp")
        # Python's tar writer adds neither AppleDouble files nor macOS xattrs.
        with tarfile.open(pending, "w:gz", format=tarfile.PAX_FORMAT) as tar:
            tar.add(package, arcname="fatima-package")
        pending.replace(opts.output / name)
        print(opts.output / name)


if __name__ == "__main__":
    build()
