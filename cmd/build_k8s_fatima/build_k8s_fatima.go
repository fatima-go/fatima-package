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
 * @date 22. 3. 9. 오후 1:35
 */

package main

import (
	"flag"
	"fmt"
	"github.com/fatima-go/fatima-package/util"
	"io/ioutil"
	"os"
	"path/filepath"
)

var usage = `usage: %s install_dir

build k8s fatima package

positional arguments:
  install_dir	compress fatima-package file saving directory
`

var (
	outputDir string
)

const (
	packageFilesDirName  = "resources/k8s"
	fatimaPackageDirName = "fatima-package"
)

func main() {
	procName := filepath.Base(os.Args[0])

	flag.Usage = func() {
		fmt.Printf(usage, procName)
	}

	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}

	outputDir = filepath.Join(flag.Args()[0])
	err := util.EnsureDirectory(outputDir, false)
	if err != nil {
		fmt.Printf("invalid install dir : %s\n", err.Error())
		return
	}

	// create tmp dir
	projectBaseDir, err := util.DetermineProjectBaseDir()
	if err != nil {
		fmt.Printf("fail to DetermineProjectBaseDir : %s\n", err.Error())
		return
	}

	tmpdir, err := ioutil.TempDir("", procName)
	if err != nil {
		fmt.Printf("fail to create temp dir : %s\n", err.Error())
		return
	}
	defer os.RemoveAll(tmpdir)

	packageFilesDir := filepath.Join(tmpdir, fatimaPackageDirName)

	// copy resources to tmp
	fmt.Printf("prepare resource files...\n")
	sourcePackageFilesDir := filepath.Join(projectBaseDir, packageFilesDirName)
	err = util.CopyDir(sourcePackageFilesDir, packageFilesDir)
	if err != nil {
		fmt.Printf("fail to prepare packageFilesDir : %s\n", err.Error())
		return
	}

	// comparess
	fmt.Printf("tar...\n")
	artifactName := fmt.Sprintf("%s-k8s", fatimaPackageDirName)
	tarFile := artifactName + ".tar"
	workingDir := filepath.Dir(packageFilesDir)
	command := fmt.Sprintf("tar cvf %s %s", tarFile, fatimaPackageDirName)
	_, err = util.ExecuteShell(workingDir, command)
	if err != nil {
		fmt.Printf("packaging artifact fail. tar error : %s\n", err.Error())
		return
	}

	fmt.Printf("compress...\n")
	command = fmt.Sprintf("gzip %s", tarFile)
	_, err = util.ExecuteShell(workingDir, command)
	if err != nil {
		fmt.Printf("packaging artifact fail. tar error : %s\n", err.Error())
		return
	}

	artifactName = tarFile + ".gz"

	oldLoc := filepath.Join(workingDir, artifactName)
	newLoc := filepath.Join(outputDir, artifactName)

	err = os.Rename(oldLoc, newLoc)
	if err != nil {
		fmt.Printf("fail to move artifact : %s\n", err.Error())
		return
	}

	// copy artifact to install_dir
	fmt.Printf("artifact ready : %s\n", artifactName)
}
