# 운영 명령 전환

기존 HTTP 핸들러와 포트는 유지한다. 새 CLI는 Jupiter/Juno의 명령별 capability를
확인한 뒤 gRPC를 사용한다. 기능 미지원 서버는 기존 HTTP 명령으로 돌아가고,
인증/연결/실행 오류는 자동으로 HTTP 재실행하지 않는다. `--legacy`는 원래 명령을
명시적으로 실행한다. TTY에서는 C안의 공통 TUI, `--plain`은 단발 출력,
`--json`은 구조화된 출력이다. 새 전용 옵션을 구형 서버가 지원하지 않으면 명시한다.

## rocron

```sh
rocron -p local01:default
rocron -p local01:default -l
rocron --json -p local01:default --process ctlprobe --job control.probe --arguments test --request-id cron-test-001
rocron --json -p local01:default --watch cron-test-001
```

TUI는 작업 선택 → 인자 입력 → 실행 확인 → 전달 결과 순서다. Enter 한 번으로
곧바로 실행하지 않는다. 결과 화면의 Enter는 같은 작업을 다시 요청하지 않는다.
`REQUESTED`는 IPC 전달 또는 `cron.rerun` 파일 기록을 의미한다. 실제 배치의 시작,
완료, 업무 성공 여부는 알 수 없으며 이를 화면에 명시한다. 배치 코드 변경은 없다.
`-l`은 기존처럼 시간대별 스케줄을 보여준다.

Juno의 `$FATIMA_HOME/data/juno/control-v2`에 요청 ID·입력 해시·이벤트·결과를 기록한다.
동일 ID와 동일 입력은 같은 결과를 반환하고, 다른 입력은 거부한다. 접속이 끊겨도
서버 작업은 계속된다. `--watch ID`나 결과 화면 `r`로 같은 요청을 재조회한다.
Juno 재기동 당시 진행 중이던 요청은 `INTERRUPTED`이며 자동으로 다시 전달하지 않는다.
목록/결과는 MONITOR, 실행은 OPERATOR 권한이 필요하다.

검증: 실제 HTTP/gRPC 혼합 포트 통합 테스트, MONITOR 실행 거부, 요청 중복 방지,
재접속/재기동 기록, TUI 실행 확인·결과 Enter 보호·60×19/80×24/132×42 화면 경계.
로컬 Mac의 `ctlprobe` 테스트 배치에 같은 ID로 두 번 요청하고 실제 실행 로그가
한 번만 기록되는 것도 확인했다.

## rostop

```sh
rostop -p local01:default ctlprobe
rostop --plain -p local01:default -g svc
rostop --json -p local01:default --watch REQUEST_ID
```

TTY에서는 대상 목록 또는 인자로 지정한 대상의 확인 화면을 연다. Enter 확인 후
goaway 진행·대기, SIGTERM 전송, 실제 PID 종료를 구독한다. Space로 복수 프로세스를
선택할 수 있다. `-g`는 한 패키지 안의 프로세스 그룹이며 `-a`는 OPM을 제외한다.
프로세스명·`-g`·`-a`를 동시에 지정하면 거부한다. 중단은 낮은 weight부터 수행한다.
이미 중단된 프로세스는 다시 신호를 보내지 않는다. 중간 실패 시 뒤의 대상은 진행하지 않는다.
새 배포/제어 경로끼리는 같은 프로세스의 동시 제어를 거부한다. 기존 HTTP 명령의
실행 방식에는 이 잠금을 추가하지 않았다.

검증: 단계별 스트림, 연결 종료 후 재구독, 같은 요청 ID의 중복 중단 방지,
실패 결과, OPM을 제외한 그룹/전체 선택, TUI 실행 전 확인.

로컬 Mac TUI에서 `ctlprobe`의 goaway 완료·SIGTERM·PID 종료 이벤트를 확인했고,
`rodis`의 DEAD 상태와 구버전 `rostart`를 통한 재기동을 확인했다.

## rostart

`rostart`도 `rostop`과 동일한 대상 선택·확인·진행 화면 및 `--plain`/`--json`/
`--watch`/`--legacy` 옵션을 제공한다. 높은 weight부터 기동하고 `startsec` 설정과
최소 3초의 생존 확인을 적용한다. 이미 살아 있는 프로세스는 다시 실행하지 않는다.
시작 중 종료되면 FAILED이며 뒤의 대상은 진행하지 않는다. 성공은 PID 생존 확인이며
애플리케이션 readiness를 검증한 것이 아님을 결과에 명시한다.

로컬 TUI에서 `ctlprobe`의 기동·PID 생존 확인을 검증했다. 즉시 종료되는 `ctlfail`은
FAILED와 exit 1을 반환하고, 요청 ID로 실패 결과를 다시 조회할 수 있음을 확인했다.

## rodis

```sh
rodis -p local01:default
rodis --plain -p local01:default -s name
rodis --json -p local01:default
```

TUI는 gRPC 상태 스트림으로 갱신한다. Juno가 기존 상태 측정 기능으로 매초 스냅샷을
만들며, 최초 구독과 매분 권한을 확인한다. Enter는 상세 정보만 연다. `/`로 이름·그룹·
상태 검색, `o`로 이름/등록 순서 정렬을 전환한다. 갱신 중 선택한 프로세스를 유지한다.
`s` 기동, `x` 중단은 별도 대상 확인 화면을 거친다. 완료 후 `n`으로 상태 화면에 복귀한다.
서버가 해당 제어 기능을 지원하지 않거나 현재 상태 구독에 오류가 있으면 바로 실행하지 않는다.
연결이 끊기면 마지막 자료와 재연결 안내가 남으며 `r`로 구독을 다시 연다.
`--plain`은 기존 주요 표/요약 항목을 한 번 출력하며 `-s name|index`를 지원한다.

로컬 Mac에서 검색 → 상세 → 중단 확인 → DEAD 갱신 → 기동 확인 → ALIVE 갱신을
실제 `ctlprobe`로 검증했다. 60×19 화면과 구버전 `rodis`의 HTTP 조회도 확인했다.
macOS 신규 조회는 고정된 결과 슬롯과 최대 8개의 동시 수집으로 프로세스 누락을
방지한다. 기존 HTTP용 Darwin 수집 코드는 변경하지 않았다.
신규 기동/중단/배포의 macOS PID 관측은 커널 sysctl을 직접 사용한다. 기존 Juno의
SIGCHLD 수거와 `ps` 명령 대기가 충돌해 살아 있는 프로세스를 DEAD로 판정하는
경쟁 조건을 신규 경로에서 피한다.
보완 후 실제 Mac에서 40회 연속 조회의 전체 목록·ALIVE 상태와 중단/기동을 확인했다.
Linux 신규 조회도 등록 설정에서 이름/그룹을 즉시 반영한다. 첫 모니터 측정 전에는
`UNKNOWN`을 표시하며, `roproc` 등록 직후 `rostart` 대상을 선택할 수 있다.

## ropack

`ropack`은 Jupiter의 패키지 목록/상태 스트림을 구독한다. `-g`는 패키지 그룹,
`-p`는 패키지 필터다. `/`로 그룹·패키지·플랫폼·상태를 검색한다. Enter는 상세,
`s`는 같은 설치 디렉터리의 `rodis`를 열며 종료하면 패키지 화면으로 돌아온다.
등록·확인 시간은 로컬 시간대다. `--plain`/`--json`은 한 번 조회한다.

Jupiter는 등록 자료의 복사본을 조회하고 기존 저장소를 변경하지 않는다. 조회자는
5초간 상태 확인 결과를 공유한다. 동시 확인은 최대 8개, 각각 2초, 전체 5초로 제한한다.
신규 Juno는 gRPC discovery, 구형 Juno는 기존 health API를 확인한다. ALIVE는 Juno API
접속 확인이며 사용자 프로세스 정상 여부를 의미하지 않는다. 주소의 패키지 ID가 다르면
MISMATCH, 접속할 수 없으면 UNREACHABLE, 확인 제한 시간을 넘으면 UNKNOWN이다.

신규/구형 Juno 혼합 목록, 주소 불일치, 조회 공유, 등록 변경 스트림, MONITOR 조회를
통합 테스트했다. 로컬 TUI의 검색·상세·rodis 이동/복귀·60×19 입력 화면과 구버전
`ropack`의 신규 Jupiter 조회도 확인했다.

## roproc

```sh
roproc -p local01:default
roproc -p local01:default add worker 4
roproc -p local01:default remove worker
roproc --json -p local01:default --watch REQUEST_ID
```

기존과 같은 Mac → Jupiter → Juno 경로다. Jupiter는 권한/패키지 정보를 확인한 뒤
요청과 스트림을 중계한다. 등록 형식, 미리보기, 변경 및 결과 저장은 Juno가 담당한다.
TUI의 Enter는 등록부 상세 조회, `a`는 등록 입력, `x`는 선택 프로세스 삭제 미리보기다.
그룹 ID 또는 이름을 입력하며 기존 기본값 4를 사용한다. 등록 시 기존 Juno 자동
기동 정책이 적용됨을 안내한다. 삭제 미리보기는 종료 여부와 프로그램·리비전·로그·
데이터·배포 이력·배치 목록의 삭제 경로를 보여준다. Enter 확인 전에는 변경하지 않는다.

미리보기 이후 설정이 달라지면 실행을 거부한다. 신규 등록 변경끼리는 잠금을 공유하고,
같은 프로세스의 신규 배포/기동/중단과도 충돌을 막는다. 삭제는 goaway와 PID 종료를
확인하고 파일을 정리한 다음 등록을 제거한다. OPM/예약 이름과 잘못된 경로는 거부한다.
기존 HTTP 핸들러는 변경하지 않았다. YAML의 다른 항목과 주석을 보존하고 임시 파일의
동기화·rename으로 저장한다. 중간 실패는 FAILED와 완료된 단계로 표시한다. 여러 파일
변경 전체가 하나의 트랜잭션인 것은 아니므로 실패 시 단계 기록을 확인해야 한다.

같은 요청 ID의 같은 요청을 서버가 다시 실행하지 않는다. 응답을 잃었으면 새 요청을
만들기 전에 `--watch ID`로 조회한다. 진행 기록 저장이 실패하면 DataLoss로 구독을
종료하며, 저장 공간과 실제 대상을 확인해야 한다. 서버 재시작 시 미완료 기록은
INTERRUPTED로 남고 자동 재실행하지 않는다.

로컬 Mac TUI에서 실제 `ctlregprobe`를 등록하고 기동한 뒤, goaway·PID 종료·모든
삭제 경로의 정리·등록 제거까지 검증했다. OPM 삭제 차단, 설정 변경 충돌, 요청 중복,
MONITOR 실행 거부, 결과 스트림 재접속, 저장 실패도 테스트했다.

## 전체 검증과 로컬 사용

전환 범위는 `rocron`, `rostop`, `rostart`, `rodis`, `ropack`, `roproc`이다.
기존 `rodeploy`는 유지하며 `rolog`, `roclric`, `rohis`, `roclip`, `rocontext`,
`roupdate`의 인터페이스는 변경하지 않는다. 사용 포트는 기존 9190/9180이다.

| Jupiter | Juno | CLI | 실제 Mac 검증 |
|---|---|---|---|
| 신규 | 신규 | 신규 | 여섯 명령 TUI 및 gRPC 기능 |
| 신규 | 신규 | 보관된 구버전 | 여섯 명령의 기존 HTTP 기능 |
| 구버전 | 구버전 | 신규 | 여섯 명령의 자동 HTTP fallback |
| 신규 | 구버전 | 신규 | 명령별 기능 확인 후 HTTP fallback |
| 구버전 | 신규 | 신규 | 기존 HTTP 인증/처리 경로 |

호환성 재검증 스크립트는 `scripts/test-operations-compatibility.py`다. 로컬 fixture의
OPM 바이너리를 일시 교체하고 `finally`에서 원래 신규 바이너리로 복원한다.
활성 context가 `local / http://127.0.0.1:9190`인지 각 제어 호출 전에 확인한다.
로그와 결과는 `.local/progressive-deploy/operations-compatibility/`에 저장한다.
단위/통합/회귀 테스트는 `scripts/test-local.sh`로 실행한다.

기본 TUI에서 종료해도 서버 작업은 계속된다. TTY의 `--watch ID`는 결과를 구독하고,
`--plain/--json --watch ID`는 현재 저장된 결과를 한 번 조회한다. 구형 서버에는
일반 기존 인자만 자동 fallback하며 `--request-id`, `--watch`, `--json` 등의 신규
보장을 제공할 수 없는 옵션은 명시적으로 거부한다.

Linux arm64의 실제 프로세스, Mac TUI, 구/신 서버 조합 및 세 Juno 순차 배포까지
검증했다. 현재 로컬 Linux 환경의 사용·재실행·Mac 복귀 방법은
[Linux arm64 로컬 테스트](linux-arm64-local-test.md)에 정리한다.
