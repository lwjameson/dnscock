<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [dnscock](#dnscock)
  - [Build](#build)
  - [Run](#run)
  - [Usage](#usage)
    - [Parameters](#parameters)
  - [DNS service discovery mechanism](#dns-service-discovery-mechanism)
  - [Differences of dnscock from tonistiigi/dnsdock](#differences-of-dnscock-from-tonistiigidnsdock)
  - [More info](#more-info)
  - [Thanks](#thanks)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# dnscock
DNS server for automatic Docker container discovery.

This project is based on https://github.com/tonistiigi/dnsdock which is in turn simplified version of https://github.com/crosbymichael/skydock.

## Build

```
$ docker build -t t0mk/dnscock ./
```

## Run

```
$ docker run -v /var/run/docker.sock:/var/run/docker.sock -p 53:53/udp t0mk/dnscock -debug=true
```

## Usage
Dnscock needs to access the docker socket to listen for events like new container creation and removal. Default value is unix socket (/var/run/docker.sock), so you should pass the socket as a volume file. You can also tell dnscock to listen on TCP socket via the "docker" parameter.

The docker-compose.yml shows basic usage. Once you run it with `docker-compose up`, you can try to query for all the containers with dig: `dig @your_ip \*.docker`.

### Parameters

Implemeted parameters with defaults:

```
-dns=":53": Listen DNS requests on this address
-docker="unix://var/run/docker.sock": Path to the docker socket
-domain="docker": Domain that is appended to all requests
-environment="": Optional context before domain suffix
-help=false: Show this message
-nameserver="8.8.8.8:53": DNS server for unmatched requests
-ttl=0: TTL for matched requests
-debug=false: Debug output
```

## DNS service discovery mechanism

Dnscock connects to Docker Remote API and keeps an up to date list of running containers. If a DNS request matches some of the containers their local IP addresses are returned.

**Format for a request matching a container is**:
`<anything>.<container-name>.<image-name>.<environment>.<domain>`.

- `environment` and `domain` are static suffixes that are set on startup. Environment is empty, domain defaults to `docker`.
- `image-name` is last part of the image tag used when starting the container.
- `container-name` alphanumerical part of container name.

You can rewrite portions of domain for a container (or even whole name) with following environment variables when running the container:

- DNSDOCK_IMAGE will rewrite iamge-name
- DNSDOCK_NAME will rewrite container-name
- DNSDOCK_TTL will rewrite default ttl from dnscock arguments
- DNSDOCK_ALIAS will create new alias for the container. The IP of the container will be resolvable by this alias. You can pass more comma-separated aliases.

You can always leave out parts from the left side. If multiple containers match then they are all returned. Wildcard requests are also supported.

Example DNS queries with example responses to illustrate the functionality:

```
$ dig @your_webserver dnscock.docker
...
;; ANSWER SECTION:
dnscock.docker.			0	IN	A	10.0.0.24
```

You might observe a 5 second delay with dig. It's under investigation. The mechanism of dig and host are a bit different from basic library gethostbyname. you should not see any delay when you `ping dnscock.docker`.

## Differences of dnscock from tonistiigi/dnsdock

- you can register container under more than one alias by passing comma-separate list to the DNSDOCK_ALIAS environment variable, e.g DNSDOCK_ALIAS="local.web1.fi,local.web2.fi"

- if you specify DNSDOCK_ALIAS=alias.some.fi environment variable to a container, dnscock will be responding on A-queries for the alias with IP of the container. You can specify same alias for more containers. Then there will be A records in the response.

- no HTTP server at the moment

- Not a difference per se, just something to pay attention: The environment variables are still called DNSDOCK_something (not DNSCOCK_something), so that you can try both projects and the invocation remains almost the same.

- Docker image is from scratch and it install a static build

## More info

More info on can be found in readme of https://github.com/tonistiigi/dnsdock.

## Thanks
Lots of code in this repo is directly influenced by skydns and skydock. Many thanks to the authors of these projects.


