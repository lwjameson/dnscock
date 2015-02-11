FROM crosbymichael/golang

ADD . /go/src/github.com/t0mk/dnscock

RUN cd /go/src/github.com/t0mk/dnscock && \
    go get -d ./... && \
    go install -ldflags "-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2>/dev/null) ]]; then echo "-dirty"; fi`" ./...

ENTRYPOINT ["/go/bin/dnscock"] 
