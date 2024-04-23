# fatima-package #
fatima-package is fatima running environment and facilities<br>
this project build fatima-package

```shell
// example running

1. cmd/build_fatima
$ go build

2. run package_build command
$ ./build_fatima ~/Downloads
```

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