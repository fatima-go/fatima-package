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
 * @date 22. 3. 4. 오후 10:58
 */

package main

import "fmt"

type supportBuild struct {
	os   string
	arch string
}

func (s supportBuild) String() string {
	return fmt.Sprintf("%s-%s", s.os, s.arch)
}

var acceptableOsList = []string{"linux", "darwin"}
var acceptableArchList = []string{"amd64", "arm64"}

var supportBuildList = []supportBuild{
	{os: "linux", arch: "amd64"},
	{os: "linux", arch: "arm64"},
	{os: "darwin", arch: "amd64"},
	{os: "darwin", arch: "arm64"},
}

// targetBuildList
func buildTarget() []supportBuild {
	if isTargetOsEmpty() && isTargetArchEmpty() {
		return supportBuildList
	}

	osList := make([]string, 0)
	if isTargetOsEmpty() {
		osList = acceptableOsList
	} else {
		if !isAcceptableOs(*targetOs) {
			fmt.Printf("unsupported os %s", *targetOs)
			return nil
		}
		osList = append(osList, *targetOs)
	}

	archList := make([]string, 0)
	if isTargetArchEmpty() {
		archList = acceptableArchList
	} else {
		if !isAcceptableArch(*targetArch) {
			fmt.Printf("unsupported arch %s", *targetArch)
			return nil
		}
		archList = append(archList, *targetArch)
	}

	targetBuildList := make([]supportBuild, 0)
	for _, os := range osList {
		for _, arch := range archList {
			targetBuildList = append(targetBuildList, supportBuild{os: os, arch: arch})
		}
	}

	return targetBuildList
}

func isAcceptableOs(os string) bool {
	for _, v := range acceptableOsList {
		if os == v {
			return true
		}
	}
	return false
}

func isAcceptableArch(arch string) bool {
	for _, v := range acceptableArchList {
		if arch == v {
			return true
		}
	}
	return false
}

func isTargetOsEmpty() bool {
	return targetOs == nil || len(*targetOs) == 0
}

func isTargetArchEmpty() bool {
	return targetArch == nil || len(*targetArch) == 0
}
