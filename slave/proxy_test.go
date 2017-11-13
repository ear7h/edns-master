package main

import (
	"testing"
	"net/http"
	"io/ioutil"
	"fmt"
	"time"
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
	m.SetQuestion("test-service.ear7h.net.", dns.TypeCNAME)

	r, err := dns.Exchange(m, _masterHost+":53")
	if err != nil {
		panic(err)
	}
	if len(r.Answer) == 0{
		panic("no answer\n" + r.String())
	}

	go serveProxy()
	go http.ListenAndServe(":8081", makeTestHandler())

	time.Sleep(2 * time.Second)

	res, err := http.Get("https://test-service.ear7h.net:4443")
	if err != nil {
		fmt.Println("GET err")
		panic(err)
	}

	byt, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	res.Body.Close()

	fmt.Println(string(byt))
}

func TestTLS(t *testing.T) {

	_localServices["test-service"] = "127.0.0.1:8081"

	post()

	m := new(dns.Msg)
	m.SetQuestion("test-service.ear7h.net.", dns.TypeCNAME)

	r, err := dns.Exchange(m, _masterHost+":53")
	if err != nil {
		panic(err)
	}
	if len(r.Answer) == 0{
		panic("no answer\n" + r.String())
	}

	go serveProxy()
	go http.ListenAndServe(":8081", makeTestHandler())

	time.Sleep(2 * time.Second)

	server := &http.Server{
		Addr: ":4443",
		Handler: makeTestHandler(),
		TLSConfig: &tls.Config{
			GetCertificate: _tlsManager.GetCertificate,
		},
	}

	go server.ListenAndServeTLS("", "")

	res, err := http.Get("https://test-service.ear7h.net:4443")
	if err != nil {
		fmt.Println("GET err")
		panic(err)
	}

	byt, _ := ioutil.ReadAll(res.Body)
	fmt.Println(string(byt))

}