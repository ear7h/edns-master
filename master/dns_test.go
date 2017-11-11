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
	go main()

	b := Block{
		Hostname: "testhost",
		Services: []string{"test-service"},
	}

	signBlock(&b)

	byt, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	_, err = http.Post("http://"+_masterIP+_masterAdminPort, "text/json", bytes.NewReader(byt))
	if err != nil {
		panic(err)
	}

	m := new(dns.Msg)
	m.SetQuestion("testhost"+".ear7h.net.", dns.TypeA)

	r, err := dns.Exchange(m, "127.0.0.1"+_dnsPort)
	if err != nil {
		panic(err)
	}

	fmt.Println("CASE 1 PASS\n", r)

	m.SetQuestion(".ear7h.net.", dns.TypeSOA)

	r, err = dns.Exchange(m, "127.0.0.1"+_dnsPort)
	if err != nil {
		panic(err)
	}

	fmt.Println(r)
}