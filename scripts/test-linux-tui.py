#!/usr/bin/env python3
"""Drive the Mac TUI against the Linux lab, including live stream reconnection."""
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
import time
import zipfile

spec = importlib.util.spec_from_file_location("checks", Path(__file__).with_name("test-linux-operations.py"))
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)
lab = c.lab
sys.path.insert(0, str(lab.BASE / "integration"))
from terminal import Terminal

OUT = lab.LAB / "tui"
passed = []


def terminal(name, *args, **kwargs):
    lab.local_guard()
    return Terminal([lab.MAC_HOME / "bin" / name, *args], OUT, name, **kwargs)


def check(label):
    passed.append(label)
    print("PASS", label, flush=True)
    (OUT / "result.json").write_text(json.dumps({"passed": passed}, indent=2) + "\n")


def main():
    OUT.mkdir(exist_ok=True)
    target = ["-p", lab.TARGET]
    with terminal("rocron", *target) as t:
        t.wait("작업 선택")
        t.wait("control.probe")
        t.save("select")
        t.send(b"\r")
        t.wait("arguments>")
        t.send(b"\x15")
        t.send((c.RUN + "-tui-cron").encode())
        t.send(b"\r")
        t.wait("내용 확인")
        t.save("review")
        t.send(b"\r")
        t.wait("실행 요청 전달 완료")
        assert "실제 배치의" in t.text()
        t.save("requested")
        t.send(b"\r")
        time.sleep(.5)
        assert c.contents("/fatima/data/ctlprobe/requests.log").splitlines().count(c.RUN + "-tui-cron") == 1
    check("rocron selection, confirmation, delivery notice and result Enter protection")

    for name in ("rostop", "rostart"):
        with terminal(name, *target, "ctlprobe", cols=100, rows=30) as t:
            t.wait("대상 확인")
            t.save("review")
            t.send(b"\r")
            t.wait("선택 대상 모두 완료", 60)
            t.save("succeeded")
            assert bool(c.live("ctlprobe")) == (name == "rostart")
    check("rostop/rostart confirmation and visible terminal completion")

    report = c.cli("rodis", *target).stdout
    assert "START TIME" in report and "GROUP" in report and "linux_arm64" in report
    check("rodis preserves its original single HTTP table report")

    for name in ("rostop", "rostart", "roproc"):
        with terminal(name, *target) as t:
            t.wait("프로세스 목록")
            t.wait("프로세스 상세")
            assert "GROUP" in t.text()
            t.save("panes")
            t.send(b"/")
            t.wait("filter>")
            t.send(b"ctlprobe")
            t.send(b"\r")
            t.wait("ctlprobe")
            t.send(b"\t")
            t.wait("[detail]")
            t.save("detail-pane")
            t.send(b"\x1b[4~")
            t.send(b"\x1b[A")
            t.resize(60, 19)
            t.wait("q 종료")
            t.save("60x19-detail")
            t.send(b"\t")
            t.wait("GROUP")
            t.save("60x19-list")
    check("process group columns, detail sheets, filtering, focus and compact layout")

    with terminal("ropack", cols=100, rows=30) as t:
        t.wait("패키지 상태 조회")
        t.wait(lab.TARGET)
        assert "GROUP" in t.text()
        t.save("sheet-list")
        t.send(b"\r")
        t.wait("[detail]")
        t.save("detail")
        t.send(b"s")
        t.wait("Enter: 패키지 목록으로 돌아가기")
        assert "START TIME" in t.text() and "GROUP" in t.text()
        t.save("rodis")
        t.send(b"\r")
        t.wait("패키지 상태 조회")
    check("ropack package detail, native rodis launch and return")

    c.copy_probe("ctluiprobe")
    with terminal("roproc", *target) as t:
        t.wait("작업 선택")
        t.send(b"a")
        t.wait("process>")
        t.send(b"ctluiprobe")
        t.send(b"\r")
        t.wait("group>")
        t.send(b"\r")
        t.wait("변경 범위 확인 · Enter")
        assert "ctluiprobe" not in c.contents("/fatima/conf/fatima-package.yaml")
        t.save("add-review")
        t.send(b"\r")
        t.wait("선택 대상 모두 완료")
        c.operation("rostart", "ctluiprobe", "ui-start")
        t.send(b"n")
        t.wait("작업 선택")
        t.send(b"/")
        t.wait("filter>")
        t.send(b"ctluiprobe")
        t.send(b"\r")
        t.send(b"x")
        t.wait("변경 범위 확인 · Enter")
        t.save("remove-review")
        t.send(b"\r")
        t.wait("선택 대상 모두 완료", 60)
        t.save("removed")
        assert not c.live("ctluiprobe")
    check("roproc TUI add, immediate start, removal preview and actual shutdown")

    far = lab.LAB / "dpv2.far"
    with terminal("rodeploy", "-g", "linux-deploy", *target, "upload", str(far)) as t:
        t.wait("FAR 확인 완료")
        t.save("upload-review")
        t.send(b"\r")
        t.wait("ARTIFACT LIST", 60)
        t.save("artifacts")
        t.send(b"\r")
        t.wait("Choose the first package")
        t.send(b"\r")
        t.wait("CONFIRM · create")
        t.save("rollout-review")
        t.send(b"\r")
        t.wait("SUCCEEDED / dpv2", 100)
        t.save("rollout-succeeded")
    artifacts = json.loads(c.cli("rodeploy", "--json", "artifacts").stdout)["artifacts"]
    assert any(a["sha256"] == hashlib.sha256(far.read_bytes()).hexdigest() and len(a["platforms"]) == 3 for a in artifacts)
    with zipfile.ZipFile(far) as archive:
        member = next(name for name in archive.namelist() if name.lstrip("/") == "platform/linux_arm64/dpv2")
        expected = hashlib.sha256(archive.read(member)).hexdigest()
    assert lab.inside("sha256sum", "/fatima/app/dpv2/dpv2").split()[0] == expected
    assert c.live("dpv2")
    check("Mac rodeploy TUI, whole multi-platform FAR upload and exact Linux arm64 installation")


if __name__ == "__main__":
    main()
