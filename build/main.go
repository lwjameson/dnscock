package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var version string

func main() {
	help := flag.Bool("help", false, "Show this message")

	config := NewConfig()

	flag.StringVar(&config.nameserver, "nameserver", config.nameserver, "DNS server for unmatched requests")
	flag.StringVar(&config.dnsAddr, "dns", config.dnsAddr, "Listen DNS requests on this address")
	domain := flag.String("domain", config.domain.String(), "Domain that is appended to all requests")
	environment := flag.String("environment", "", "Optional context before domain suffix")
	flag.StringVar(&config.dockerHost, "docker", config.dockerHost, "Path to the docker socket")
	flag.BoolVar(&config.verbose, "verbose", true, "Verbose output")
	flag.BoolVar(&config.debug, "debug", false, "See coming queries")
	flag.IntVar(&config.ttl, "ttl", config.ttl, "TTL for matched requests")

	var showVersion bool
	if len(version) > 0 {
		flag.BoolVar(&showVersion, "version", false, "Show application version")
	}

	flag.Parse()

	if showVersion {
		fmt.Println("dnscock", version)
		return
	}

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	config.domain = NewDomain(*environment + "." + *domain)

	dnsServer := NewDNSServer(config)

	docker, err := NewDockerManager(config, dnsServer)
	if err != nil {
		log.Fatal(err)
	}
	if err := docker.Start(); err != nil {
		log.Fatal(err)
	}

	if err := dnsServer.Start(); err != nil {
		log.Fatal(err)
	}

}
