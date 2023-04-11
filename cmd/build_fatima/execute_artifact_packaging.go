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
 * @date 22. 2. 28. 오후 10:00
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"throosea/fatima-package/util"
)

type InjectPackagingArtifact struct {
}

func (i InjectPackagingArtifact) Name() string {
	return "packaing artifact"
}

func (i InjectPackagingArtifact) GetSteps() []string {
	return []string{
		"make tar",
		"compress",
		"artifact ready",
	}
}

func (i InjectPackagingArtifact) Execute(jobContext *JobContext, stepper StepIncrementer) error {
	stepper.Incr()
	tarFile := jobContext.artifact + ".tar"
	workingDir := filepath.Dir(jobContext.packageFilesDir)
	command := fmt.Sprintf("tar cvf %s %s", tarFile, fatimaPackageDirName)
	_, err := util.ExecuteShell(workingDir, command)
	//fmt.Printf("%s\n", out)
	if err != nil {
		return fmt.Errorf("packaging artifact fail. tar error : %s", err.Error())
	}

	//fmt.Printf("compress %s...\n", tarFile)
	stepper.Incr()
	command = fmt.Sprintf("gzip %s", tarFile)
	_, err = util.ExecuteShell(workingDir, command)
	//fmt.Printf("%s\n", out)
	if err != nil {
		return fmt.Errorf("packaging artifact fail. tar error : %s", err.Error())
	}

	jobContext.artifact = tarFile + ".gz"

	oldLoc := filepath.Join(workingDir, jobContext.artifact)
	newLoc := filepath.Join(jobContext.outputDir, jobContext.artifact)

	//fmt.Printf("moving artifact to %s\n", newLoc)
	err = os.Rename(oldLoc, newLoc)
	if err != nil {
		return fmt.Errorf("fail to move artifact : %s", err.Error())
	}

	stepper.Incr()

	return nil
}
