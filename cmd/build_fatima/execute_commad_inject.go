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
 * @date 22. 2. 28. 오후 9:58
 */

package main

import (
	"fmt"
	"github.com/fatima-go/fatima-package/util"
	"gopkg.in/src-d/go-git.v4"
	"net/url"
	"path/filepath"
)

const (
	githubUrlFatimaBinaries = "https://github.com/fatima-go/fatima-cmd.git"
)

func buildSourceUrl(urlpath, token string) string {
	if len(token) == 0 {
		return urlpath
	}

	surl, _ := url.Parse(urlpath)
	surl.User = url.User(token)
	return surl.String()
}

type InjectCommand struct {
}

func (i InjectCommand) Name() string {
	return "inject fatima binaries"
}

func (i InjectCommand) GetSteps() []string {
	return []string{
		"cloning fatima-cmd",
		"go module verifying",
		"build binaries",
	}
}

func (i InjectCommand) Execute(jobContext *JobContext, stepper StepIncrementer) error {
	progressLogger.Log("[%s] cloning fatima-cmd : %s", jobContext.target, githubUrlFatimaBinaries)
	stepper.Incr()
	sourceUrl := buildSourceUrl(githubUrlFatimaBinaries, jobContext.sourceUrlToken)
	workingDir, _ := jobContext.CreateDir("fatima-cmd")
	_, err := git.PlainClone(workingDir, false, &git.CloneOptions{
		URL:      sourceUrl,
		Progress: nil,
		//Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("clone fail : %s", err.Error())
	}

	progressLogger.Log("[%s] go module verifying...", jobContext.target)
	stepper.Incr()
	err = util.GoModuleVerify(workingDir)
	if err != nil {
		return err
	}

	installBinDir := filepath.Join(jobContext.packageFilesDir, "bin")
	command := fmt.Sprintf("./build.sh -o %s -a %s -d %s",
		jobContext.target.os,
		jobContext.target.arch,
		installBinDir,
	)
	stepper.Incr()
	out, err := util.ExecuteCommand(workingDir, command)
	progressLogger.Log("[%s] fatima-cmd build\n%s", jobContext.target, out)
	if err != nil {
		return fmt.Errorf("fail to execute command. build script return error : %s", err.Error())
	}

	return nil
}
