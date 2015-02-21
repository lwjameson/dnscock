package main

import (
	"errors"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Service struct {
	Name  string
	Image string
	Ip    net.IP
	Ttl   int
	Alias string
}

func NewService() (s *Service) {
	s = &Service{Ttl: -1, Alias: ""}
	return
}

type ServiceListProvider interface {
	AddService(string, Service)
	RemoveService(string) error
	GetService(string) (Service, error)
	GetAllServices() map[string]Service
}

type DNSServer struct {
	config   *Config
	server   *dns.Server
	services map[string]*Service
	aliases  map[string]map[string]struct{}
	lock     *sync.RWMutex
}

func NewDNSServer(c *Config) *DNSServer {
	s := &DNSServer{
		config:   c,
		services: make(map[string]*Service),
		aliases:  make(map[string]map[string]struct{}),
		lock:     &sync.RWMutex{},
	}

	mux := dns.NewServeMux()
	//mux.HandleFunc(c.domain[len(c.domain)-1]+".", s.handleRequest)
	//mux.HandleFunc(".", s.forwardRequest)
	mux.HandleFunc(".", s.handleRequest)

	s.server = &dns.Server{Addr: c.dnsAddr, Net: "udp", Handler: mux}

	return s
}

func (s *DNSServer) IsLocal(name string) bool {
	// Is it really this easy? I haven't read the RFCs.
	return strings.HasSuffix(name, s.config.domain.String())
}

func (s *DNSServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *DNSServer) Stop() {
	s.server.Shutdown()
}

// This is copypasted from golang/src/net. Once they export it, I can remove
// it.
func isDomainName(s string) bool {
	// See RFC 1035, RFC 3696.
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}

func (s *DNSServer) AddAlias(alias string, id string) {
	ok := isDomainName(alias)
	if ok {
		// assign service id to alias. If there's no map for the alias key,
		// create it
		id_map, ok := s.aliases[alias]
		if !ok {
			id_map = make(map[string]struct{})
			s.aliases[alias] = id_map
			log.Println("Created new entry for alias, id:", alias, id)
		} else {
			log.Println("Add another entry for alias, id:", alias, id)
		}
		id_map[id] = struct{}{}
	} else {
		log.Println(alias, "passed as an Alias is not a valid domain name. Not using it.")
	}
}

func (s *DNSServer) AddService(id string, service Service) {
	defer s.lock.Unlock()
	s.lock.Lock()

	id = s.getExpandedId(id)
	s.services[id] = &service

	if service.Alias != "" {
		for _, alias := range strings.Split(service.Alias, ",") {
			s.AddAlias(alias, id)
		}
	}

	if s.config.verbose {
		log.Println("Added service:", id, service)
	}
}

func (s *DNSServer) RemoveAliasesForId(id string) error {
	for alias, id_map := range s.aliases {
		// go through all existing aliases and check if they point to removed
		// id. If yes, remove the linking.
		if _, alias_pointed := id_map[id]; alias_pointed {
			delete(id_map, id)
			log.Println("Deleted id from alias", id, alias)
		}
		// if this was the last id for the alias, remove the alias completely
		if len(id_map) == 0 {
			delete(s.aliases, alias)
			log.Println("Removed empty alias", alias)
		}
	}
	return nil
}

func (s *DNSServer) RemoveService(id string) error {
	defer s.lock.Unlock()
	s.lock.Lock()

	id = s.getExpandedId(id)
	if _, ok := s.services[id]; !ok {
		return errors.New("No such service: " + id)
	}

	s.RemoveAliasesForId(id)

	delete(s.services, id)

	if s.config.verbose {
		log.Println("Stopped service:", id)
	}

	return nil
}

func (s *DNSServer) GetService(id string) (Service, error) {
	defer s.lock.RUnlock()
	s.lock.RLock()

	id = s.getExpandedId(id)
	if s, ok := s.services[id]; !ok {
		// Check for a pa
		return *new(Service), errors.New("No such service: " + id)
	} else {
		return *s, nil
	}
}

func (s *DNSServer) GetAllServices() map[string]Service {
	defer s.lock.RUnlock()
	s.lock.RLock()

	list := make(map[string]Service, len(s.services))
	for id, service := range s.services {
		list[id] = *service
	}

	return list
}

func (s *DNSServer) forwardRequest(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	if in, _, err := c.Exchange(r, s.config.nameserver); err != nil {
		log.Print(err)
		w.WriteMsg(new(dns.Msg))
	} else {
		w.WriteMsg(in)
	}
}

func getServiceRecord(s *Service, name string, default_ttl int) *dns.A {
	rr := new(dns.A)
	var ttl int
	if s.Ttl != -1 {
		ttl = s.Ttl
	} else {
		ttl = default_ttl
	}
	rr.Hdr = dns.RR_Header{
		Name:   name,
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    uint32(ttl),
	}
	rr.A = s.Ip
	return rr
}

func (s *DNSServer) getServicesForAlias(alias string) (pointed []*Service) {

	defer s.lock.RUnlock()
	s.lock.RLock()

	for service_id := range s.aliases[alias] {
		pointed = append(pointed, s.services[service_id])
	}
	return pointed
}

func (s *DNSServer) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	if s.config.debug {
		log.Println("incoming query", r)
	}

	query := r.Question[0].Name

	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	if query == "print-status" {
		log.Println("services: ", s.services)
		log.Println("aliases: ", s.aliases)
	}

	alias_id_map, alias_exists := s.aliases[query]
	if alias_exists {
		if r.Question[0].Qtype == dns.TypeA {
			if s.config.debug {
				log.Println("A query for existing alias, getting all the pointed services for A records in reply")
			}
			m.Answer = make([]dns.RR, 0, len(alias_id_map))
			relevant_services := s.getServicesForAlias(query)

			for i := range relevant_services {
				m.Answer = append(m.Answer,
					getServiceRecord(
						relevant_services[i], r.Question[0].Name, s.config.ttl))
			}
			w.WriteMsg(m)
			return
		} else {
			if s.config.debug {
				log.Println("non-A query for existing alias, return SOA")
			}
			m.Answer = s.createSOA()
			w.WriteMsg(m)
			return
		}
	}

	if !s.IsLocal(query) {
		if s.config.debug {
			log.Println("query is not local, forwarding")
		}
		s.forwardRequest(w, r)
		return
	}
	if s.config.debug {
		log.Println("This query is for local domain", s.config.domain)
	}

	if r.Question[0].Qtype != dns.TypeA {
		if s.config.debug {
			log.Println("Non-A query. We service only A in local domain, reply with SOA")
		}
		m.Answer = s.createSOA()
		w.WriteMsg(m)
		return
	}

	m.Answer = make([]dns.RR, 0, 2)

	for service := range s.queryServices(query) {
		m.Answer = append(m.Answer,
			getServiceRecord(service, r.Question[0].Name, s.config.ttl))
	}
	if len(m.Answer) == 0 {
		m.Answer = s.createSOA()
	}

	w.WriteMsg(m)
}

func (s *DNSServer) queryServices(query string) chan *Service {
	c := make(chan *Service)

	go func() {
		query := strings.Split(strings.ToLower(query), ".")

		defer s.lock.RUnlock()
		s.lock.RLock()

		for _, service := range s.services {
			tests := [][]string{
				s.config.domain,
				strings.Split(service.Image, "."),
				strings.Split(service.Name, "."),
			}

			for i, q := 0, query; ; i++ {
				if len(q) == 0 || i > 2 {
					c <- service
					break
				}

				var matches bool
				if matches, q = matchSuffix(q, tests[i]); !matches {
					break
				}
			}

		}

		close(c)

	}()

	return c

}

// Checks for a partial match for container SHA and outputs it if found.
func (s *DNSServer) getExpandedId(in string) (out string) {
	out = in

	// Hard to make a judgement on small image names.
	if len(in) < 4 {
		return
	}

	if isHex, _ := regexp.MatchString("^[0-9a-f]+$", in); !isHex {
		return
	}

	for id, _ := range s.services {
		if len(id) == 64 {
			if isHex, _ := regexp.MatchString("^[0-9a-f]+$", id); isHex {
				if strings.HasPrefix(id, in) {
					out = id
					return
				}
			}
		}
	}
	return
}

// Ttl is used from config so that not-found result responses are not cached
// for a long time. The other defaults left as is(skydns source) because they
// do not have an use case in this situation.
func (s *DNSServer) createSOA() []dns.RR {
	dom := dns.Fqdn(s.config.domain[len(s.config.domain)-1] + ".")
	soa := &dns.SOA{Hdr: dns.RR_Header{Name: dom, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: uint32(s.config.ttl)},
		Ns:      "master." + dom,
		Mbox:    "hostmaster." + dom,
		Serial:  uint32(time.Now().Truncate(time.Hour).Unix()),
		Refresh: 28800,
		Retry:   7200,
		Expire:  604800,
		Minttl:  uint32(s.config.ttl),
	}
	return []dns.RR{soa}
}

func matchSuffix(str, sfx []string) (matches bool, remainder []string) {
	for i := 1; i <= len(sfx); i++ {
		if len(str) < i {
			return true, nil
		}
		if sfx[len(sfx)-i] != str[len(str)-i] && str[len(str)-i] != "*" {
			return false, nil
		}
	}
	return true, str[:len(str)-len(sfx)]
}
