FROM golang:onbuild

EXPOSE 53/udp

ENTRYPOINT ["/go/src/app/dnscock"] 
