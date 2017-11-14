package main

import (
	"testing"
	"net/http"
	"io/ioutil"
	"fmt"
	"crypto/tls"
	"github.com/miekg/dns"
)

func makeTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}
}

func TestProxy(t *testing.T) {

	_localServices["test-service"] = "127.0.0.1:8081"

	post()

	m := new(dns.Msg)
	m.SetQuestion(_hostname+".ear7h.net.", dns.TypeCNAME)

	r, err := dns.Exchange(m, _masterHost+":53")
	if err != nil {
		panic(err)
	}

	registerOk := false
	for _, v := range r.Answer {
		if v.Header().Rrtype == dns.TypeA {
			registerOk = true
			break
		}
	}

	if !registerOk {
		panic("did not register properly\n" + r.String())
	}


	server := &http.Server{
		Addr: ":4443",
		Handler: makeTestHandler(),
		TLSConfig: &tls.Config{
			GetCertificate: _tlsManager.GetCertificate,
		},
	}

	go server.ListenAndServe()
	//go server.ListenAndServeTLS("", "")

	res, err := http.Get("http://"+_hostname+".ear7h.net:4443")
	if err != nil {
		fmt.Println("GET err")
		panic(err)
	}

	byt, _ := ioutil.ReadAll(res.Body)
	fmt.Println(string(byt))

	if string(byt) != "hello" {
		t.Fail()
	}

}