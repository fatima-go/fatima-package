#!/bin/sh

#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with p work for additional information
# regarding copyright ownership.  The ASF licenses p file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use p file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# @project fatima
# @author DeockJin Chung (jin.freestyle@gmail.com)
# @date 22. 3. 9. 오후 2:30
#

# check OS
darwin=false
case "`uname`" in
Darwin*) darwin=true;;
esac

# 1. check far submit (for proc)
fatima_package="fatima-package"
proc=""
basedir=$PWD
if [ "$darwin" = false ]; then
    basedir=$(dirname $(readlink -f $0))
fi

appscandir="${basedir}/${fatima_package}/app"
for entry in "$appscandir"/*
do
    proc="$(basename ${entry})"
    break
done

if [ "$proc" == "" ];then
    echo "not found process"
    exit 1
fi

echo "process name : $proc"

export PATH=$PATH:.
export FATIMA_HOME=$basedir/$fatima_package

appdir=$FATIMA_HOME/app/$proc

# 2. find pid
pidfile=$appdir/proc/$proc.pid
if [ ! -f $pidfile ]; then
    echo "not found $proc pidfile"
    exit 1
fi

pid=`cat $pidfile`
echo "pid : $pid"

# 3. send goaway signal
echo "sending goaway signal..."
kill -SIGUSR1 $pid

goawayfile=$appdir/goaway.sh
if [ -f $goawayfile ]; then
    echo "executing goaway.sh..."
    cd $appdir
    exec goaway.sh
fi

# 4. send term
echo "sending TERM signal..."
kill -SIGTERM $pid

echo "shutdown finished.."