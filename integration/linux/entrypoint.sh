#!/bin/sh
set -eu
if [ ! -f /fatima/packing-info.json ]; then
    cp -a /opt/distribution/. /fatima/
    cp -a /opt/fixture/. /fatima/
fi
# Apply each newly built distribution without overwriting runtime configuration.
cp -a /opt/distribution/bin/. /fatima/bin/
for program in jupiter juno saturn; do
    cp /opt/distribution/app/$program/$program /fatima/app/$program/$program
done
cp /opt/distribution/packing-info.json /fatima/packing-info.json
mkdir -p /root/.fatima
if [ ! -f /root/.fatima/config ]; then
    rocontext -l http://127.0.0.1:9190 -u admin -p admin add local
    rocontext use local
fi
stop() {
    stopro -y
    sleep 3
    exit 0
}
trap stop TERM INT
startro -y
while :; do sleep 1 & wait $! || true; done
