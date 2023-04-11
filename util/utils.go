/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with p work for additional information
 * regarding copyright ownership.  The ASF licenses p file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use p file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 * @project fatima
 * @author DeockJin Chung (jin.freestyle@gmail.com)
 * @date 22. 3. 9. 오후 1:53
 */

package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
)

const (
	resourceDirname = "resources"
	cmdDirname      = "cmd"
	gitDirname      = ".git"
	gitConfigfile   = "config"
	procTypeGeneral = "GENERAL"
	procTypeUI      = "USER_INTERACTIVE"
)

var ErrGopathNotFound = fmt.Errorf("not found GOPATH. (gopath is nil)")
var ErrGitNotFound = fmt.Errorf("not found git")

func FindGitConfig(dir string) (string, error) {
	gopath := os.Getenv("GOPATH")
	if len(gopath) == 0 {
		return "", ErrGopathNotFound
	}

	if dir == filepath.Join(gopath, "src") {
		return "", ErrGitNotFound
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	// find .git
	for _, file := range files {
		if file.Name() == gitDirname {
			if file.IsDir() && EnsureFileInDirectory(filepath.Join(dir, file.Name()), gitConfigfile) {
				return dir, nil
			}
		}
	}

	parentDir := filepath.Dir(dir)
	return FindGitConfig(parentDir)
}

func EnsureFileInDirectory(dir, targetFilename string) bool {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fail to read dir %s : %s", dir, err.Error())
		return false
	}

	// find .git
	for _, file := range files {
		if file.Name() == targetFilename {
			if !file.IsDir() {
				return true
			}
			fmt.Fprintf(os.Stderr, "%s found but it is directory", dir)
			return false
		}
	}

	// not found
	return false
}

// find project base dir
func DetermineProjectBaseDir() (string, error) {
	currentWd, _ := os.Getwd()
	foundBaseDir, err := FindGitConfig(currentWd)
	if err != nil {
		return foundBaseDir, err
	}

	return foundBaseDir, nil
}

// Dir copies a whole directory recursively
func CopyDir(src string, dst string) error {
	var err error
	var fds []os.FileInfo
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = CopyFile(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

// File copies a single file from src to dst
func CopyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func ExecuteCommand(workingDir, command string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("empty command")
	}

	var cmd *exec.Cmd
	s := regexp.MustCompile("\\s+").Split(command, -1)
	i := len(s)
	if i == 0 {
		return "", errors.New("empty command")
	} else if i == 1 {
		cmd = exec.Command(s[0])
	} else {
		cmd = exec.Command(s[0], s[1:]...)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = workingDir
	err := cmd.Run()
	if err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

func ExecuteShell(workingDir, command string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("empty command")
	}

	var cmd *exec.Cmd
	cmd = exec.Command("/bin/sh", "-c", command)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = workingDir
	err := cmd.Run()
	if err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

func GoModuleVerify(workingDir string) error {
	command := fmt.Sprintf("go mod tidy")
	out, err := ExecuteCommand(workingDir, command)
	if err != nil {
		fmt.Printf("%s\n", out)
		return fmt.Errorf("go module verify fail")
	}
	return nil
}

func EnsureDirectory(path string, forceCreate bool) error {
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if forceCreate {
				return os.MkdirAll(path, 0755)
			}
		} else if !stat.IsDir() {
			return errors.New(fmt.Sprintf("%s path exist as file", path))
		}
	}

	return nil
}
