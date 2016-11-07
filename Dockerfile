FROM golang

ADD . /usr/src/statsd-collector-mongo
WORKDIR /usr/src/statsd-collector-mongo

RUN ./build.sh

ENTRYPOINT /usr/src/statsd-collector-mongo/statsd-collector-mongo
