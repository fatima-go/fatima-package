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
 * @date 22. 2. 28. 오후 9:12
 */

package main

import (
	"fmt"
	"github.com/fatima-go/fatima-package/util"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	packageFilesDirName  = "resources/standard"
	fatimaPackageDirName = "fatima-package"
)

type JobContext struct {
	projectBaseDir  string
	target          supportBuild
	outputDir       string
	packageFilesDir string // dir : .../{maybe tempDir}/fatima-package
	tempDir         string // dir : system temporary dir
	sourceUrlToken  string
	artifact        string
	executorList    []PackagingExecutor
	steps           []string
	bar             *uiprogress.Bar
}

type PackingInfo struct {
	User      string `json:"user"`
	BuildTime string `json:"build_time"`
}

func NewPackingInfo() PackingInfo {
	p := PackingInfo{}
	// find author
	user, err := ExecuteShell(".", "whoami")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whoami error : %s\n", err.Error())
		user = "unknown"
	}

	p.User = strings.TrimSpace(user)

	zoneName, _ := time.Now().Zone()
	p.BuildTime = time.Now().Format(yyyyMMddHHmmss) + " " + zoneName
	return p
}

const (
	yyyyMMddHHmmss = "2006-01-02 15:04:05"
)

func (j *JobContext) String() string {
	return j.target.String()
}

func (j *JobContext) Incr() {
	j.bar.Incr()
}

func (j *JobContext) Prepend(b *uiprogress.Bar) string {
	info := fmt.Sprintf("%s: %s", j, j.steps[b.Current()-1])
	return strutil.Resize(info, 32)
}

func (j *JobContext) Print() {
	fmt.Println("-------------------------------------")
	fmt.Printf("target=%s\n", j.target)
	fmt.Printf("projectBaseDir=%s\n", j.projectBaseDir)
	fmt.Printf("packageFilesDir=%s\n", j.packageFilesDir)
	fmt.Printf("outputDir=%s\n", j.outputDir)
	fmt.Println("-------------------------------------")
}

func (j *JobContext) GetWorkingDir() string {
	return j.tempDir
}

func (j *JobContext) CreateDir(dirname string) (string, error) {
	dir := filepath.Join(j.tempDir, dirname)
	return dir, os.Mkdir(dir, 0755)
}

func (j *JobContext) Close() {
	os.RemoveAll(filepath.Dir(j.packageFilesDir))
}

func createJobContext(target supportBuild) (*JobContext, error) {
	var err error
	ctx := &JobContext{}
	ctx.projectBaseDir, err = util.DetermineProjectBaseDir()
	if err != nil {
		return ctx, fmt.Errorf("fail to DetermineProjectBaseDir : %s", err.Error())
	}

	tmpdir, err := os.MkdirTemp("", "fatima-package")
	if err != nil {
		return ctx, fmt.Errorf("fail to create temp dir : %s", err.Error())
	}

	ctx.sourceUrlToken = *pat //"MUST_BE_PROVIDED_AS_ARG" // TODO
	ctx.tempDir = tmpdir
	ctx.packageFilesDir = filepath.Join(tmpdir, fatimaPackageDirName)
	ctx.target = target
	ctx.outputDir = outputDir
	sourcePackageFilesDir := filepath.Join(ctx.projectBaseDir, packageFilesDirName)

	// prepare files...
	os.MkdirAll(ctx.packageFilesDir, 0755)
	err = util.CopyDir(sourcePackageFilesDir, ctx.packageFilesDir)
	if err != nil {
		return ctx, fmt.Errorf("fail to prepare packageFilesDir : %s", err.Error())
	}

	ctx.artifact = fmt.Sprintf("%s.%s", fatimaPackageDirName, ctx.target)

	ctx.executorList = make([]PackagingExecutor, 0)
	ctx.executorList = append(ctx.executorList, InjectCommand{})
	ctx.executorList = append(ctx.executorList, InjectOPM{})
	ctx.executorList = append(ctx.executorList, InjectPackingInfo{})
	ctx.executorList = append(ctx.executorList, InjectPackagingArtifact{})
	ctx.steps = make([]string, 0)
	for _, executor := range ctx.executorList {
		ctx.steps = append(ctx.steps, executor.GetSteps()...)
	}

	ctx.bar = uiprogress.AddBar(len(ctx.steps)).AppendCompleted().PrependElapsed()
	ctx.bar.Width = 50
	ctx.bar.PrependFunc(ctx.Prepend)

	return ctx, nil
}
