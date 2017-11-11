package main

import (
	"testing"
	"github.com/miekg/dns"
	"fmt"
)

func init() {
}

func TestPost(t *testing.T) {
	_localServices["test-service"] = "127.0.0.1:8080"

	post()

	m := new(dns.Msg)
	m.SetQuestion("test-service.ear7h.net.", dns.TypeCNAME)

	r, err := dns.Exchange(m, _masterHost+":53")
	if err != nil {
		panic(err)
	}

	fmt.Println(r)
}