FROM alpine:3.22
RUN apk add --no-cache bash procps lsof tzdata
COPY fatima-package.linux-arm64.tar.gz /tmp/package.tar.gz
RUN mkdir -p /opt/distribution && tar -xzf /tmp/package.tar.gz -C /opt/distribution --strip-components=1 && rm /tmp/package.tar.gz
COPY fixture/ /opt/fixture/
COPY entrypoint.sh /opt/entrypoint.sh
ENV FATIMA_HOME=/fatima
ENV PATH=/fatima/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:.
WORKDIR /fatima
ENTRYPOINT ["/bin/sh", "/opt/entrypoint.sh"]
