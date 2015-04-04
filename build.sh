#!/bin/sh

docker build -t dnscock-builder build
docker run -id dnscock-builder
ID=`docker ps -ql`
rm -rf ship/dnscock
docker cp ${ID}:/go/src/app/dnscock ship/
docker rm -f $ID
docker build -t t0mk/dnscock ship
