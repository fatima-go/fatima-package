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
 * @date 22. 10. 7. 오후 5:34
 */

package main

import (
	"bytes"
	"errors"
	"os/exec"
)

func ExecuteShell(wd, command string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("empty command")
	}

	var cmd *exec.Cmd
	cmd = exec.Command("/bin/sh", "-c", command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = wd
	err := cmd.Run()
	if err != nil {
		return out.String(), err
	}
	return out.String(), nil
}
