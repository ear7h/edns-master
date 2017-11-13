package main

import (
	"testing"
	"encoding/json"
	"net/http"
	"bytes"
	"github.com/miekg/dns"
	"fmt"
)

func TestDNS(t *testing.T) {
	//go main()
	host := "ear7h.net"

	b := Block{
		Hostname: "testhost",
		Services: []string{"test-service"},
	}

	signBlock(&b)

	byt, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	_, err = http.Post("http://"+host+_masterAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		panic(err)
	}

	m := new(dns.Msg)
	m.SetQuestion("testhost"+".ear7h.net.", dns.TypeA)

	r, err := dns.Exchange(m, host+":53")
	if err != nil {
		panic(err)
	}

	fmt.Println(r)

	m.SetQuestion("test-service.ear7h.net.", dns.TypeCNAME)

	r, err = dns.Exchange(m, host+":53")
	if err != nil {
		panic(err)
	}

	fmt.Println(r)
}