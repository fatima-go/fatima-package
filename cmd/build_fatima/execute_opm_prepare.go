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
 * @date 22. 2. 28. 오후 9:59
 */

package main

import (
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"net/url"
	"path/filepath"
	"throosea/fatima-package/util"
)

type OpmProcess struct {
	Name      string
	SourceUrl string
}

func (o OpmProcess) BuildSourceUrl(token string) string {
	if len(token) == 0 {
		return o.SourceUrl
	}

	surl, _ := url.Parse(o.SourceUrl)
	surl.User = url.User(token)
	return surl.String()
}

var opmProccessList = []OpmProcess{
	{Name: "jupiter", SourceUrl: "https://github.com/throosea/jupiter.git"},
	{Name: "juno", SourceUrl: "https://github.com/throosea/juno.git"},
	{Name: "saturn", SourceUrl: "https://github.com/throosea/saturn.git"},
}

type InjectOPM struct {
}

func (i InjectOPM) Name() string {
	return "inject fatima opm processes"
}

func (i InjectOPM) GetSteps() []string {
	processList := make([]string, 0)
	for _, process := range opmProccessList {
		name := process.Name
		processList = append(processList, fmt.Sprintf("%s cloning", name))
		processList = append(processList, fmt.Sprintf("%s go module verifying", name))
		processList = append(processList, fmt.Sprintf("%s build", name))
	}
	return processList
}

func (i InjectOPM) Execute(jobContext *JobContext, stepper StepIncrementer) error {
	for _, process := range opmProccessList {
		err := buildOpmProcess(process, jobContext, stepper)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildOpmProcess(target OpmProcess, jobContext *JobContext, stepper StepIncrementer) error {
	//fmt.Printf("cloning %s...\n", target.Name)
	stepper.Incr()
	workingDir, _ := jobContext.CreateDir(target.Name)
	sourceUrl := target.BuildSourceUrl(jobContext.sourceUrlToken)
	_, err := git.PlainClone(workingDir, false, &git.CloneOptions{
		URL:      sourceUrl,
		Progress: nil,
		//Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("clone fail : %s", err.Error())
	}

	// go build -o /tmp/juno
	//fmt.Println("go module verifying...")
	stepper.Incr()
	err = util.GoModuleVerify(workingDir)
	if err != nil {
		return err
	}

	installBinDir := filepath.Join(jobContext.packageFilesDir, "app", target.Name)
	command := fmt.Sprintf("GOOS=%s GOARCH=%s go build -ldflags=\"-s -w\" -o %s",
		jobContext.target.os,
		jobContext.target.arch,
		installBinDir,
	)

	stepper.Incr()
	_, err = util.ExecuteShell(workingDir, command)
	//fmt.Printf("%s\n", out)
	if err != nil {
		return fmt.Errorf("building %s return error : %s", target.Name, err.Error())
	}

	return nil
}
