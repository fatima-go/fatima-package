# Mac CLI → Linux arm64 검증 환경

2026-09-10 검증 환경은 Colima의 실제 Linux aarch64 커널(6.8.0)과 Alpine 3.22 기반
컨테이너다. Mac의 CLI에서 Jupiter/Juno에 접속하며, 컨테이너 안의 Linux CLI도 실행한다.
Mac OPM은 중지하고 컨테이너 OPM을 기존 `local` 주소로 제공한다.

| 항목 | 값 |
|---|---|
| rocontext | `local`, `http://127.0.0.1:9190` |
| Juno | `http://127.0.0.1:9180` |
| 패키지 / 그룹 | `linux01:default` / `linux-deploy` |
| 컨테이너 | `fatima-operations-local` |
| 컨테이너 FATIMA_HOME | `/fatima` |
| 영속 Docker volume | `fatima-operations-home` |
| 테스트 프로세스 | `ctlprobe`(배치/제어), `dpv2`(배포) |

컨테이너는 Colima **Linux VM의 host network**를 사용한다. OPM은 VM의 loopback에서
9190/9180을 listen하고 Colima가 Mac의 같은 포트로 연결한다. Juno가 등록하는
`127.0.0.1:9180` 주소를 Mac과 Jupiter 양쪽에서 그대로 사용할 수 있다.
추가 OPM 포트, `/etc/hosts` 수정, 애플리케이션 프록시는 필요 없다.
이 구성은 Colima용이며 다른 Docker 실행 환경에서는 포트 연결 방식을 확인해야 한다.

## 바로 사용

```sh
rocontext use local
ropack
rodis -p linux01:default
rocron -p linux01:default
rostop -p linux01:default ctlprobe
rostart -p linux01:default ctlprobe
roproc -p linux01:default
```

테스트 FAR에는 `darwin_arm64`, `linux_arm64`, `linux_amd64` 실행 파일이 들어 있다.

```sh
fatima_linux_lab="/Users/dave_01/IronForge/fatima-go/src/fatima-package/.local/progressive-deploy/linux-operations"
rodeploy -g linux-deploy -p linux01:default upload "$fatima_linux_lab/dpv2.far"
```

`ctlprobe`의 `control.probe`를 요청하면 `/fatima/data/ctlprobe/requests.log`에 인자가 남는다.
`rocron`의 전달 완료는 실제 배치 완료를 보장하지 않는다. 테스트에서는 이 로그까지
별도로 확인해 같은 요청 ID가 한 번만 전달되는 것을 검증한다.

```sh
docker --context colima exec fatima-operations-local cat /fatima/data/ctlprobe/requests.log
docker --context colima exec fatima-operations-local tail -n 60 /fatima/log/juno/juno.log
```

## 기동·중지·Mac 복귀

`fatima-package` 디렉터리에서 실행한다. Mac의 활성 context가 `local / 127.0.0.1:9190`인지
확인하며, Docker 호출은 Colima 소켓을 명시해 다른 Docker context의 서버를 사용하지 않는다.

```sh
python3 scripts/linux-local.py status
python3 scripts/linux-local.py up
python3 scripts/linux-local.py down
python3 scripts/linux-local.py restore-mac
```

`up`은 필요하면 기존 Colima를 시작하고, `fatima-download`의 Linux arm64 tar.gz로
이미지를 만든다. Mac OPM을 중지한 뒤 Linux 컨테이너를 시작한다. 초기 구성과 테스트
프로그램/FAR도 생성한다. 이후 기동에서는 CLI/OPM 실행 파일을 갱신하고 volume의 설정,
배포본, 요청 기록은 보존한다. 컨테이너를 다시 만들면 테스트 프로세스는 다음처럼 기동한다.

```sh
rostart --plain -p linux01:default ctlprobe
# dpv2를 한 번 배포한 환경에서 사용
rostart --plain -p linux01:default dpv2
```

`down`은 컨테이너만 중지한다. `restore-mac`은 컨테이너를 중지하고 Mac OPM을 재기동한다.
이때 패키지는 다시 `local01:default / local-deploy`다. Mac `$FATIMA_HOME`과 Linux volume은
분리되어 있으며 컨테이너 파일로 Mac 프로그램을 덮어쓰지 않는다.

## 자동 검증

```sh
python3 scripts/test-linux-operations.py
python3 scripts/test-linux-operations-compatibility.py
.local/progressive-deploy/ui-venv/bin/python3 scripts/test-linux-tui.py
```

TUI 검증은 `pyte`가 설치된 Python 환경을 사용한다. 현재 Mac에는 위 가상환경이 준비되어 있다.
호환성 테스트는 보관된 `.local/progressive-deploy/legacy`의 Mac CLI와
`.local/progressive-deploy/linux/legacy`의 Linux Jupiter/Juno를 사용한다.
테스트는 `ctlprobe`를 중단/기동하고 임시 프로세스를 등록/삭제한다. 호환성 테스트는
OPM 바이너리를 일시 교체하고 `finally`에서 신규 버전으로 복원한다.

| 검증 | 결과 |
|---|---|
| Mac 및 Linux CLI의 여섯 운영 명령 | 통과 |
| 배치 IPC 전달, 같은 ID 중복 방지, 다른 입력 충돌 | 통과 |
| goaway, 실제 PID 종료, 시작 생존 확인, 즉시 종료 실패 | 통과 |
| 등록 직후 기동, 삭제 미리보기, 프로세스/리비전/로그/데이터 삭제 | 통과 |
| 연속 상태 조회, 실제 Linux PID 대조 | 통과 |
| 클라이언트 종료 후 작업 완료/재조회 | 통과 |
| Juno 강제 종료 후 INTERRUPTED 보존, 자동 재실행 금지 | 통과 |
| 여섯 TUI, 결과 Enter 재실행 방지, rodis 재연결 | 통과 |
| 구 CLI → 신 서버 및 구/신 Jupiter·Juno 혼합 네 조합 | 통과 |
| 다중 플랫폼 FAR 전체 업로드 및 Linux arm64 실행 파일 해시 일치 | 통과 |
| 세 Juno에 업로드 한 번, 첫 배포 대기, 나머지 순차 배포, 재배포 goaway | 통과 |
| 구형 배포 CLI 및 신규 배포 fallback 네 조합 | 통과 |
| Jupiter/Juno 배포 저장소 장애 시 신규 배포 차단, 기존 HTTP 유지 | 통과 |
| Linux에서 core 요청 저장/Juno service/API 통합 테스트 실행 | 통과 |

세 Juno 검증은 기존 `scripts/test-linux.py`를 Docker 내부 네트워크에서 실행한다.
수동 테스트용 `local` 컨테이너와 별개이며 종료 시 해당 테스트 컨테이너를 정리한다.
결과와 명령 로그는 `.local/progressive-deploy/linux-operations/`에,
TUI 화면은 그 안의 `tui/`에 저장한다.

검증 중 최소 Linux 이미지의 시간대 데이터 누락을 확인해 이미지에 `tzdata`를 추가했다.
또한 신규 Linux API가 아직 모니터에 수집되지 않은 등록 항목을 누락하던 문제를 수정했다.
등록 이름/그룹은 설정에서 즉시 읽고, 첫 측정 전 상태는 `UNKNOWN`으로 표시한다.
기존 HTTP 핸들러와 모니터 수집 동작은 변경하지 않았다.

OA/VPN/LB를 지나는 실제 서버 경로와 운영 서버의 OS/설정 조합은 별도 단계에서 검증한다.
