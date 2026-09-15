# fatima-package #
fatima-package is fatima running environment and facilities<br>
this project build fatima-package

형제 프로젝트의 현재 작업 파일로 빌드·설치하는 신규 개발 경로는
[신규 배포 기능 사용 및 검증](docs/progressive-deploy-usage.md)을 참고한다.

```shell
// example running

1. cmd/build_fatima
$ go build

2. run package_build command
$ ./build_fatima ~/Downloads
```

# GitHub Personal Access Token #
fatima-cmd 및 OPM(jupiter / juno / saturn) 소스를 github.com에서 clone하기 위해 GitHub Personal Access Token이 필요할 수 있다 (private repo 접근 또는 rate limit 회피 목적).

토큰은 다음 두 가지 방법 중 하나로 제공할 수 있으며, **-t 옵션이 환경변수보다 우선**한다.

### 1. 환경변수로 제공 (권장)
```shell
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
$ ./build_fatima -o linux -a amd64 ~/Downloads
```

### 2. -t 옵션으로 직접 전달
```shell
$ ./build_fatima -o linux -a amd64 -t ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx ~/Downloads
```

두 값 모두 제공하지 않으면 토큰 없이 clone을 시도한다 (public repo는 정상 동작).

# 유의 사항 #
## OSX에서 압축한걸 linux 에서 풀때 "._" 파일 생성 이슈 ##
(특히) Apple silicon 맥북에서 패키징을 통해 tar 로 파일을 묶은 파일을 linux 시스템에서 풀때 아래와 같은 경고 문구와 함께 수많은 "._" 로 시작하는 파일들이 생성될 수 있다.
```shell
tar: Ignoring unknown extended header keyword `LIBARCHIVE.xattr.com.apple.provenance'
```

원인은 맥북은 BSD 버전의 tar를 사용하고 리눅스에서는 gnu 버전의 tar를 사용하면서 gnu 버전에서는 알 수 없는 속성이기 때문이다.

아래와 같이 ._fatima-package 식으로 모든 하위 디렉토리에 생성된다. 

```shell
$ ls -la
total 49372
drwxrwxr-x 3 djin.chung djin.chung       93 Apr 23 16:07 .
drwx------ 3 djin.chung djin.chung      152 Apr 23 16:01 ..
-rwxr-xr-x 1 djin.chung djin.chung      163 Apr 23 16:06 ._fatima-package
drwxr-xr-x 7 djin.chung djin.chung      190 Apr 23 16:06 fatima-package
-rw-r--r-- 1 root       root       50550786 Apr 23 16:06 fatima-package.linux-arm64.tar.gz
```

## 해결책 ##
gnu tar 를 설치해서 쓰는걸 권장한다

```shell
# tar 설치
brew install gnu-tar
```
만약 gtar 가 설치되지 않아도 기본적으로 WARNING 만 띄우고 실제 "._" 파일이 생성되지 않도록 내부적으로 tar 압축시에 아래와 같은 옵션을 사용한다.
```golang
COPYFILE_DISABLE=1 
```
따라서 더 이상 "._" 파일은 생기진 않지만 압축 풀때 경고 문구는 나오기에 무시하도록 한다.

## Core v2 / standalone OPM build

- Runtime: `github.com/fatima-go/fatima-core/v2 v2.0.0`
- Operations: `github.com/fatima-go/fatima-opm v1.0.0`
- Juno, Jupiter and CLI revisions are pinned in go.mod. Production module builds do not require local replace directives.
- Local source builds use `go.work.dev` and include the sibling `fatima-opm` repository. Saturn remains on core v1; separate executables may use different runtime major versions.

```sh
./scripts/test-local.sh
python3 scripts/verify-module-compatibility.py
python3 scripts/build-local.py --os linux --arch arm64 --output .local/module-split
```

The compatibility script downloads pinned released modules, starts only temporary local processes, verifies new Juno Goaway/IPC against core v1.3.4/v1.3.7 IPC services, and checks old/new OPM gRPC schema interoperability in both directions. It uses minimal runtime environment fixtures; it is not a full application deployment test. It does not alter an installed Fatima environment or user context.

### Migration verification

Runtime IPC and HTTP/gRPC service names remain unchanged. Old/new protobuf packages must never be linked into the same executable. Check dependency closure, not only direct imports. Existing deployment journals retain their JSON shape and location; restart/cancellation coverage is provided by Jupiter/Juno and CLI integration suites.

Core's existing environment-dependent IPC unit test failure is tracked in its release notes. Docker runtime smoke tests require an available Docker daemon and are separate from successful cross-platform package builds.
