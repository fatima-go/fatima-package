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
 * @date 22. 2. 28. 오후 8:35
 */

package main

import (
	"flag"
	"fmt"
	"github.com/fatima-go/fatima-package/util"
	"github.com/gosuri/uiprogress"
	"os"
	"path/filepath"
	"sync"
)

var usage = `usage: %s -o linux -a amd64 -t my_personal_access_token install_dir

build fatima package

positional arguments:
  -o os        target os. e.g) linux darwin
  -a arch      target arch. e.g) amd64 arm64
  -t token      github.com personal access token
  install_dir	compress fatima-package file saving directory
`

var (
	targetOs   = flag.String("o", "", "target os. e.g) linux darwin")
	targetArch = flag.String("a", "", "target arch. e.g) amd64 arm64")
	pat        = flag.String("t", "", "github PAT")
	outputDir  string
)

type StepIncrementer interface {
	Incr()
}

type PackagingExecutor interface {
	Name() string
	Execute(ctx *JobContext, step StepIncrementer) error
	GetSteps() []string
}

var progressLogger = NewLogger()

func main() {
	validateGolang()

	flag.Usage = func() {
		fmt.Printf(usage, os.Args[0])
	}

	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.Usage()
		return
	}

	defer progressLogger.Close()

	outputDir = filepath.Join(flag.Args()[0])
	err := util.EnsureDirectory(outputDir, false)
	if err != nil {
		fmt.Printf("invalid install dir : %s", err.Error())
		return
	}

	targetBuildList := buildTarget()
	if len(targetBuildList) == 0 {
		fmt.Printf("there is no target....\n")
		return
	}

	defer PrintReport()

	uiprogress.Start()

	wg := sync.WaitGroup{}
	wg.Add(len(targetBuildList))
	for _, target := range targetBuildList {
		go buildFatimaPackage(&wg, target)
	}

	wg.Wait()

	uiprogress.Stop()

	fmt.Println("")
}

func buildFatimaPackage(wg *sync.WaitGroup, target supportBuild) {
	defer wg.Done()

	err := packagingJob(target)
	if err != nil {
		Report(fmt.Sprintf("%s : %s", target, err.Error()))
	}
}

func packagingJob(target supportBuild) error {
	jobContext, err := createJobContext(target)
	if err != nil {
		return fmt.Errorf("create context fail : %s\n", err.Error())
	}
	defer jobContext.Close()

	for _, executor := range jobContext.executorList {
		err = executor.Execute(jobContext, jobContext)
		if err != nil {
			return fmt.Errorf("[%s] %s", executor.Name(), err.Error())
		}
	}

	Report(fmt.Sprintf("%s : artifact packaged => %s", jobContext, jobContext.artifact))
	return nil
}

func validateGolang() {
	command := fmt.Sprintf("go version")
	out, err := util.ExecuteCommand(".", command)
	if err != nil {
		fmt.Println("you have to install golang on your system first")
		os.Exit(0)
	}

	fmt.Printf("using : %s\n", out)
}
