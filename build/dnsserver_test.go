package main

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNSResponse(t *testing.T) {
	const TEST_ADDR = "127.0.0.1:9953"

	config := NewConfig()
	config.dnsAddr = TEST_ADDR

	server := NewDNSServer(config)
	go server.Start()

	// Allow some time for server to start
	time.Sleep(250 * time.Millisecond)

	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = []dns.Question{
		dns.Question{"google.com.", dns.TypeA, dns.ClassINET},
	}
	c := new(dns.Client)
	in, _, err := c.Exchange(m, TEST_ADDR)

	if err != nil {
		t.Error("Error response from the server", err)
	}

	if len(in.Answer) < 3 {
		t.Error("DNS request only responded ", len(in.Answer), "answers")
	}

	server.AddService("foo", Service{Name: "foo", Image: "bar", Ip: net.ParseIP("127.0.0.1")})
	server.AddService("baz", Service{Name: "baz", Image: "bar", Ip: net.ParseIP("127.0.0.1"), Ttl: -1, Alias: "www.seznam.cz"})

	server.AddService("erw", Service{Name: "erw", Image: "bar", Ip: net.ParseIP("127.0.0.1"), Ttl: -1, Alias: "www.seznam.cz,www.someelse.fi"})

	var inputs = []struct {
		query    string
		expected int
	}{
		{"docker.", 3},
		{"*.docker.", 3},
		{"bar.docker.", 3},
		{"foo.docker.", 0},
		{"baz.bar.docker.", 1},
		{"www.seznam.cz.", 2},
		{"www.someelse.fi.", 1},
	}

	for _, input := range inputs {
		t.Log(input.query)
		m := new(dns.Msg)
		m.Id = dns.Id()
		m.RecursionDesired = true
		m.Question = []dns.Question{
			dns.Question{input.query, dns.TypeA, dns.ClassINET},
		}
		c := new(dns.Client)
		in, _, err := c.Exchange(m, TEST_ADDR)
		if err != nil {
			t.Error("Error response from the server", err)
			break
		}
		if len(in.Answer) == 0 {
			t.Error(input, "No SOA anwer")
		}
		if _, ok := in.Answer[0].(*dns.SOA); ok {
			if input.expected != 0 {
				t.Error(input, "Expected:", input.expected, " Got:", 0)
			}
		} else if len(in.Answer) != input.expected {
			t.Error(input, "Expected:", input.expected, " Got:", len(in.Answer))
		}
	}

	// // This test is slow and pointless
	// server.Stop()
	//
	// c = new(dns.Client)
	// _, _, err = c.Exchange(m, TEST_ADDR)
	//
	// if err == nil {
	// 	t.Error("Server still running but should be shut down.")
	// }
}

func TestServiceManagement(t *testing.T) {
	list := ServiceListProvider(NewDNSServer(NewConfig()))

	if len(list.GetAllServices()) != 0 {
		t.Error("Initial service count should be 0.")
	}

	A := Service{Name: "bar"}
	list.AddService("foo", A)

	if len(list.GetAllServices()) != 1 {
		t.Error("Service count should be 1.")
	}

	A.Name = "baz"

	s1, err := list.GetService("foo")
	if err != nil {
		t.Error("GetService error", err)
	}

	if s1.Name != "bar" {
		t.Error("Expected: bar got:", s1.Name)
	}

	_, err = list.GetService("boo")

	if err == nil {
		t.Error("Request to boo should have failed")
	}

	list.AddService("boo", Service{Name: "boo"})

	all := list.GetAllServices()

	delete(all, "foo")
	s2 := all["boo"]
	s2.Name = "zoo"

	if len(list.GetAllServices()) != 2 {
		t.Error("Local map change should not remove items")
	}

	if s1, _ = list.GetService("boo"); s1.Name != "boo" {
		t.Error("Local map change should not change items")
	}

	err = list.RemoveService("bar")
	if err == nil {
		t.Error("Removing bar should fail")
	}

	err = list.RemoveService("foo")
	if err != nil {
		t.Error("Removing foo failed", err)
	}

	if len(list.GetAllServices()) != 1 {
		t.Error("Item count after remove should be 1")
	}

	list.AddService("416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df", Service{Name: "mysql"})

	if s1, _ = list.GetService("416261"); s1.Name != "mysql" {
		t.Error("Container can't be found by prefix")
	}

	err = list.RemoveService("416261")
	if err != nil {
		t.Error("Removing 416261 failed", err)
	}

	if len(list.GetAllServices()) != 1 {
		t.Error("Item count after remove should be 1")
	}

}

func TestAliasManagement(t *testing.T) {
	servId := "fsdfsdfsd"
	servId2 := "fdsfsdfsdsdfsdfsd"
	alias := "www.seznam.cz"
	alias2 := "www.chmi.cz"
	s := NewDNSServer(NewConfig())
	s.AddService(servId, Service{Name: "mysql", Alias: alias})
	id_map, exists := s.aliases[alias]
	if len(id_map) != 1 {
		t.Error("should be one service under", alias)
	}
	_, id_is_there := id_map[servId]
	if !id_is_there {
		t.Error("Service alias registered under wrong id")
	}
	if !exists {
		t.Error("alias entry was not created")
	}
	s.AddService(servId2, Service{Name: "redis", Alias: alias + "," + alias2})
	id_map, exists = s.aliases[alias]
	if len(id_map) != 2 {
		t.Error("should be two services now under alias", alias)
	}

	id_map, exists = s.aliases[alias2]
	if len(id_map) != 1 {
		t.Error("There should be one service under alias", alias2)
	}

	s.RemoveService(servId)

	id_map, exists = s.aliases[alias]
	if len(id_map) != 1 {
		t.Error("should be again one service under alias", alias)
	}
	_, id_is_there = id_map[servId2]
	if !id_is_there {
		t.Error("service alias registered under wrong id")
	}

	s.RemoveService(servId2)
	if len(s.aliases) != 0 {
		t.Error("aliases map should be empty")
	}
	// invalid domain name
	s.AddService(servId, Service{Name: "mysql", Alias: "$not_a_domain.99"})

	if len(s.aliases) != 0 {
		t.Error("invalid domain name alias should not be considered")
	}

}

func TestDNSRequestMatch(t *testing.T) {
	server := NewDNSServer(NewConfig())

	server.AddService("foo", Service{Name: "foo", Image: "bar"})
	server.AddService("baz", Service{Name: "baz", Image: "bar"})
	server.AddService("abc", Service{Name: "def", Image: "ghi"})

	inputs := []struct {
		query, domain string
		expected      int
	}{
		{"docker", "docker", 3},
		{"baz.docker", "docker.local", 0},
		{"docker.local", "docker.local", 3},
		{"foo.docker.local", "docker.local", 0},
		{"bar.docker.local", "docker.local", 2},
		{"foo.bar.docker.local", "docker.local", 1},
		{"*.local", "docker.local", 3},
		{"*.docker.local", "docker.local", 3},
		{"bar.*.local", "docker.local", 2},
		{"*.*.local", "docker.local", 3},
		{"foo.*.local", "docker.local", 0},
		{"bar.*.docker.local", "docker.local", 0},
		{"foo.*.docker", "docker", 1},
		{"baz.foo.bar.docker.local", "docker.local", 1},
		{"foo.bar", "baz.foo.bar", 3},
	}

	for _, input := range inputs {
		server.config.domain = NewDomain(input.domain)

		t.Log(input.query, input.domain)

		actual := 0
		for _ = range server.queryServices(input.query) {
			actual++
		}

		if actual != input.expected {
			t.Error(input, "Expected:", input.expected, "Got:", actual)
		}
	}
}

func TestDNSRequestMatchNamesWithDots(t *testing.T) {
	server := NewDNSServer(NewConfig())

	server.AddService("boo", Service{Name: "foo.boo", Image: "bar.zar"})
	server.AddService("baz", Service{Name: "baz", Image: "bar.zar"})
	server.AddService("abc", Service{Name: "bar", Image: "zar"})

	inputs := []struct {
		query, domain string
		expected      int
	}{
		{"foo.boo.bar.zar.docker", "docker", 2},
		{"zar.docker", "docker", 3},
		{"*.docker", "docker", 3},
		{"baz.bar.zar.docker", "docker", 2},
		{"boo.bar.zar.docker", "docker", 2},
		{"coo.bar.zar.docker", "docker", 1},
	}

	for _, input := range inputs {
		server.config.domain = NewDomain(input.domain)

		t.Log(input.query, input.domain)

		actual := 0
		for _ = range server.queryServices(input.query) {
			actual++
		}

		if actual != input.expected {
			t.Error(input, "Expected:", input.expected, "Got:", actual)
		}
	}
}

func TestGetExpandedId(t *testing.T) {
	server := NewDNSServer(NewConfig())

	server.AddService("416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df", Service{})
	server.AddService("316261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f14nothex", Service{})
	server.AddService("abcdefabcdef", Service{})

	inputs := map[string]string{
		"416":          "416",
		"41626":        "416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df",
		"416261e74515": "416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df",
		"31626":        "31626",
		"abcde":        "abcde",
		"foobar":       "foobar",
	}

	for input, expected := range inputs {
		if actual := server.getExpandedId(input); actual != expected {
			t.Error(input, "Expected:", expected, "Got:", actual)
		}
	}

}
