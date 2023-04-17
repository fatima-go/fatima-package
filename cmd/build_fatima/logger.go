/*
 * Copyright 2023 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @project fatima-core
 * @author jin
 * @date 23. 4. 17. 오후 11:26
 */

package main

import (
	"fmt"
	"os"
	"time"
)

type ProgressLogger interface {
	Log(v ...interface{})
	Close() error
}

type defaultProgressLogger struct {
	logFile *os.File
}

func (d *defaultProgressLogger) Log(v ...interface{}) {
	if d.logFile == nil {
		return
	}

	var express string
	if format, ok := v[0].(string); ok {
		express = fmt.Sprintf(format, v[1:]...)
	} else {
		express = fmt.Sprintf("%v", v[0])
	}

	_, _ = d.logFile.WriteString(fmt.Sprintf("%s\n", express))
}

func (d *defaultProgressLogger) Close() error {
	if d.logFile == nil {
		return nil
	}

	return d.logFile.Close()
}

const TimeYyyymmddhhmmss = "2006.01.02-15.04.00"

func NewLogger() ProgressLogger {
	logger := &defaultProgressLogger{}
	filename := fmt.Sprintf("/tmp/build_fatima.log.%s", time.Now().Format(TimeYyyymmddhhmmss))
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("fail to create logger (%s) : %s", filename, err.Error())
		return logger // return empty logger
	}
	logger.logFile = file
	fmt.Printf("progress log file prepared : %s\n", filename)
	return logger
}
