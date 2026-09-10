# 구버전 rodeploy → 신규 Jupiter/Juno 로컬 테스트

이 문서는 Mac OPM의 `local01:default` 기준이다. Linux 컨테이너 검증 환경을 실행 중이면
대상은 `linux01:default`이며, [Linux 테스트 가이드](linux-arm64-local-test.md)를 따른다.
Mac 환경으로 돌아오려면 `fatima-package`에서 `python3 scripts/linux-local.py restore-mac`을 실행한다.

현재 Mac에서 사용 가능한 준비물:

- 신규 Jupiter: `http://127.0.0.1:9190`, 신규 Juno: `http://127.0.0.1:9180`
- rocontext: `local`, 그룹: `local-deploy`, 패키지: `local01:default`
- 테스트 프로세스: `dpv2` (등록 및 실행 중)
- 실제 구버전 rodeploy: `.local/progressive-deploy/legacy/rodeploy`
- 테스트 FAR: `.local/progressive-deploy/gopath/far/dpv2/dpv2.far`

2026-09-10 기준으로 두 서버의 capabilities에서 `api_version: 2`를 확인했다.
구버전 실행 파일은 보관된 Darwin ARM64 원본 패키지의 파일과 SHA-256이 동일하다.
내장 모듈 버전은 `v0.0.0-20260831073523-dc8bac561376`이다.

## 1. 접속 대상과 배포 전 상태 확인

```sh
fatima_test_dir="/Users/dave_01/IronForge/fatima-go/src/fatima-package/.local/progressive-deploy"
rocontext use local
ropack
rodis -p local01:default
readlink "$FATIMA_HOME/app/dpv2"
```

`ropack`에서 endpoint가 localhost인지 확인하고, `rodis`의 `dpv2` PID와 START TIME,
`readlink`의 revision 경로를 기억해 둔다.

서버의 신규 기능 제공 여부도 직접 볼 수 있다.

```sh
curl -fsS http://127.0.0.1:9190/.well-known/fatima/capabilities
curl -fsS http://127.0.0.1:9180/.well-known/fatima/capabilities
```

## 2. 실제 구버전 바이너리로 배포

아래 명령은 테스트용 `dpv2`를 기존 HTTP 경로로 재배포한다.

```sh
"$fatima_test_dir/legacy/rodeploy" \
  -p local01 \
  "$fatima_test_dir/gopath/far/dpv2/dpv2.far"
```

이 구버전은 멀티 플랫폼 FAR의 플랫폼을 찾을 때 `-p` 값을 호스트명과 그대로 비교한다.
따라서 도움말의 `host:package` 형식대로 `-p local01:default`를 사용하면
`fail to find platform : not found deploy by host local01:default`로 업로드 전에 종료된다.
현재 테스트 패키지는 `default`이므로 `-p local01`을 사용한다. Jupiter는 패키지명을
생략한 `local01`을 `local01:default`로 해석한다. `default` 이외의 패키지에는 이 생략
방식을 적용할 수 없다. 상태 확인용 `rodis -p local01:default`는 그대로 사용한다.

예전 형식의 `login success`, `start transfer`와 전송 점, `waiting server response`,
`target : 1 juno enqueued`가 표시되는지 확인한다. TUI는 열리지 않는다.
그룹 지정 방식은 동일한 명령에서 `-p local01`을 `-g local-deploy`로 바꿔 시험한다.
현재 로컬 그룹에는 패키지가 하나다.

## 3. 설치 및 재기동 확인

명령이 반환된 뒤 다음을 확인한다.

```sh
rodis -p local01:default
readlink "$FATIMA_HOME/app/dpv2"
tail -n 60 "$FATIMA_HOME/log/juno/juno.log"
tail -n 40 "$FATIMA_HOME/log/dpv2/dpv2.log"
```

정상 판정 기준:

- `dpv2`가 `ALIVE`이며 PID와 시작 시각이 배포 전과 달라진다.
- `app/dpv2`의 revision 링크가 새 revision으로 바뀐다.
- Juno 로그에 goaway/종료/기동과 배포 이력이 보이고, 설치·기동 실패 로그가 없다.
- Jupiter/Juno/Saturn은 계속 `ALIVE`다.

기존 `enqueued` 응답이나 종료 코드 0만으로 성공을 판정하지 않는다. 구형 CLI는 오류를
출력하고도 0으로 종료할 수 있고, Jupiter의 기존 응답에는 Juno별 최종 결과가 포함되지 않는다.
`dpv2`가 아직 전환 중이면 잠시 후 `rodis`로 다시 확인한다.

진행 로그를 계속 보려면 별도 터미널에서 아래 명령을 먼저 실행한다.

```sh
tail -F "$FATIMA_HOME/log/juno/juno.log" "$FATIMA_HOME/log/dpv2/dpv2.log"
```

일반 `rodeploy`는 설치된 최신 CLI를 계속 사용한다. 이 테스트는 보관된 구버전 실행 파일을
절대 경로로 호출하며, 서버나 FATIMA_HOME을 구버전으로 교체할 필요가 없다.
