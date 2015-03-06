FROM golang:onbuild

RUN go build -o dnscock

EXPOSE 53/udp

ENTRYPOINT ["/go/src/app/dnscock"] 
