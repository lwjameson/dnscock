<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [dnscock](#dnscock)
  - [Usage](#usage)
    - [Parameters](#parameters)
  - [DNS service discovery mechanism](#dns-service-discovery-mechanism)
  - [Differences of dnscock from tonistiigi/dnsdock](#differences-of-dnscock-from-tonistiigidnsdock)
  - [Differences of dnsdock|dnscock from skydock](#differences-of-dnsdock|dnscock-from-skydock)
  - [OSX Usage](#osx-usage)
- [](#)
    - [Lots of code in this repo is directly influenced by skydns and skydock. Many thanks to the authors of these projects.](#lots-of-code-in-this-repo-is-directly-influenced-by-skydns-and-skydock-many-thanks-to-the-authors-of-these-projects)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## dnscock
DNS server for automatic Docker container discovery.

This project is based on https://github.com/tonistiigi/dnsdock which is in turn simplified version of https://github.com/crosbymichael/skydock.

### Usage
Dnscock needs to access the docker socket to listen for events like new container creation and removal. Default value is unix socket (/var/run/docker.sock), so you should pass the socket as a volume file. You can also tell dnscock to listen on TCP socket via the "docker" parameter.

The docker-compose.yml shows basic usage. Once you run it with `docker-compose up`, you can try to query for all the containers with dig: `dig @your_ip \*.docker`.

#### Parameters

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

### DNS service discovery mechanism

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
- DNSDOCK_alias will create new alias for the container. The IP of the container will be resolvable by this alias. You can pass more comma-separated aliases.

You can always leave out parts from the left side. If multiple containers match then they are all returned. Wildcard requests are also supported.

Example DNS queries with example responses to illustrate the functionality:

```
> dig *.docker
...
;; ANSWER SECTION:
docker.			0	IN	A	172.17.42.5
docker.			0	IN	A	172.17.42.3
docker.			0	IN	A	172.17.42.2
docker.			0	IN	A	172.17.42.7

> dig redis.docker
...
;; ANSWER SECTION:
redis.docker.		0	IN	A	172.17.42.3
redis.docker.		0	IN	A	172.17.42.2

> dig redis1.redis.docker
...
;; ANSWER SECTION:
redis1.redis.docker.		0	IN	A	172.17.42.2

> dig redis1.*.docker
...
;; ANSWER SECTION:
redis1.*.docker.		0	IN	A	172.17.42.2
```

### Differences of dnscock from tonistiigi/dnsdock

- you can register container under more than one alias by passing comma-separate list to the DNSDOCK_ALIAS environment variable, e.g DNSDOCK_ALIAS="local.web1.fi,local.web2.fi"

- if you specify DNSDOCK_ALIAS=alias.some.fi environment variable to a container, dnscock will be responding on A-queries for the alias with IP of the container. You can specify same alias for more containers. Then there will be A records in the response.

- no HTTP server at the moment

- Not a difference per se, just something to pay attention: The environment variables are still called DNSDOCK_something (not DNSCOCK_something), so that you can try both projects and the invocation remains almost the same.

- Docker image is from scratch and it install a static build

### Differences of dnsdock|dnscock from skydock

- *No raft / simple in-memory storage* - Does not use any distributed storage and is meant to be used only inside single host. This means no ever-growing log files and memory leakage. AFAIK skydock currently does not have a state machine so the raft log always keeps growing and you have to recreate the server periodically if you wish to run it for a long period of time. Also the startup is very slow because it has to read in all the previous log files.

- *No TTL heartbeat* - Skydock sends heartbeats for every container that reset the DNS TTL value. In production this has not turned out to be reliable. What makes this worse it that if a heartbeat has been missed, skydock does not recover until you restart it. Dnsdock uses static TTL that does not count down. You can override it for a container and also change it without restarting(before updates). In most cases you would want to use TTL=0 anyway.

- *No dependency to other container* - Dnsdock does not use a separate DNS server but has one built in. Linking to another container makes recovery from crash much harder. For example skydock does not recover from skydns crash even if the crashed container is restarted.

- A records only for now.

- No support for Javascript plugins.

- There's a slight difference in a way image names are extracted from a container. Skydock uses the last tag set on image while dnsdock uses the specific tag that was used when the container was created. This means that if a new version of an image comes out and untags the image that your container still uses, the DNS requests for this old container still work.

### OSX Usage

Original tutorial: http://www.asbjornenge.com/wwc/vagrant_skydocking.html

If you use docker on OSX via Vagrant you can do this to make your containers discoverable from your main machine.

In your Vagrantfile add the following to let your virtual machine accept packets for other IPs:

```ruby
config.vm.provider :virtualbox do |vb|
  vb.customize ["modifyvm", :id, "--nicpromisc2", "allow-all"]
end
```

Then route traffic that matches you containers to your virtual machine internal IP:

```
sudo route -n add -net 172.17.0.0 <VAGRANT_MACHINE_IP>
```

Finally, to make OSX use dnscock for requests that match your domain suffix create a file with your domain ending under `/etc/resolver` (for example `/etc/resolver/myprojectname.docker`) and set its contents to `nameserver 172.17.42.1`.

---

#### Lots of code in this repo is directly influenced by skydns and skydock. Many thanks to the authors of these projects.


