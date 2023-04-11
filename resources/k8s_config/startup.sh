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
# @date 22. 3. 9. 오후 2:27
#

# check argument
if [ $# -ne 2 ]
then
    echo "invalid argument please pass loglevel and profile for argument "
    exit
fi

####################################################
# environment
# check OS
darwin=false
case "`uname`" in
Darwin*) darwin=true;;
esac

# check param
loglevel=$1
loglevelcode="0x1F"

case "$loglevel" in
error) loglevelcode="0x7";;
warn) loglevelcode="0xF";;
info) loglevelcode="0x1F";;
debug) loglevelcode="0x2F";;
trace) loglevelcode="0xFF";;
esac

profile=$2
####################################################

####################################################
# startup process
# 1. check far submit (for proc)
fatima_package="fatima-package"
farfile=""
farname=""
proc=""
basedir=$PWD
if [ "$darwin" = false ]; then
    basedir=$(dirname $(readlink -f $0))
fi

for entry in "$basedir"/*
do
    ext="${entry##*.}"
    if [ $ext == "far" ]; then
        farfile=$entry
        farname=$(basename "$entry")
        proc="${farname%.*}"
        break
    fi
done

if [ "$proc" == "" ];then
    echo "not found process"
    exit 1
fi

echo "submit found : $farname"
echo "process name : $proc"

export PATH=$PATH:.
export FATIMA_HOME=$basedir/$fatima_package
export FATIMA_USERNAME=admin
export FATIMA_PASSWORD=admin
export FATIMA_PROFILE=$profile
export FATIMA_JUPITER_URI=http://127.0.0.1:9190
export FATIMA_TIMEZONE=Asia/Seoul

appdir=$FATIMA_HOME/app/$proc

# 2. install package
echo "install package..."
mkdir -p $appdir
cp $farfile $appdir
cd $appdir
unzip $farname
rm -rf $basedir/$farname

# 3. configure proc
echo "configure process..."
cd $FATIMA_HOME/conf
echo "  - gid       : 4" >> fatima-package.yaml
echo "    name      : $proc" >> fatima-package.yaml
echo "    loglevel  : $loglevel" >> fatima-package.yaml

cd $FATIMA_HOME/package/cfm
echo "{ \"${proc}\": \"${loglevelcode}\" }" > loglevels

cd $appdir
chmod +x $appdir/goaway.sh >& /dev/null

# 4. execute proc
cd $appdir
echo "starting $proc ..."
exec $proc