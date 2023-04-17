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
 * @date 22. 10. 7. 오후 5:37
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type InjectPackingInfo struct {
}

func (i InjectPackingInfo) Name() string {
	return "inject packing info"
}

func (i InjectPackingInfo) GetSteps() []string {
	return []string{
		"create packing info",
	}
}

const (
	packingFileName = "packing-info.json"
)

func (i InjectPackingInfo) Execute(jobContext *JobContext, stepper StepIncrementer) error {
	stepper.Incr()
	b, err := json.Marshal(NewPackingInfo())
	if err != nil {
		return fmt.Errorf("fail to marshal packing info : %s\n", err.Error())
	}

	progressLogger.Log("[%s] inject packing info\n%s", jobContext.target, string(b))

	packingFilePath := filepath.Join(jobContext.packageFilesDir, packingFileName)
	err = os.WriteFile(packingFilePath, b, 0644)
	if err != nil {
		return fmt.Errorf("fail to write %s file : %s\n", packingFileName, err.Error())
	}

	return nil
}
