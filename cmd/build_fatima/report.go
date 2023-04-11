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
 * @date 22. 3. 4. 오후 10:25
 */

package main

import (
	"fmt"
	"sync"
)

var reportList = make([]string, 0)

var rm sync.Mutex

func Report(info string) {
	rm.Lock()
	defer rm.Unlock()

	reportList = append(reportList, info)
}

func PrintReport() {
	if len(reportList) == 0 {
		return
	}

	for _, v := range reportList {
		fmt.Printf("%s\n", v)
	}
}
