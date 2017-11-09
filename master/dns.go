package main

import (
	"fmt"
	"github.com/miekg/dns"
	"time"
)

func serial() uint32 {
	return uint32(time.Now().Sub(_start) / (_timeout * time.Second))
}

func serveDNS() error {
	ret := make(chan error, 1)

	dns.HandleFunc(".", makeDNSHandler())
	go func() {
		srv := &dns.Server{Addr: _dnsPort, Net: "udp"}
		ret <- srv.ListenAndServe()

	}()
	go func() {
		srv := &dns.Server{Addr: _dnsPort, Net: "tcp"}
		ret <- srv.ListenAndServe()
	}()

	return <-ret
}

func soa() []dns.RR {
	return []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   _domainDot,
				Rrtype: dns.TypeSOA,
				Class:  dns.ClassINET,
				Ttl:    uint32(_timeout),
			},
			Ns:      "ns1." + _domainDot,
			Mbox:    "julio.grillo98.gmail.com",
			Serial:  serial(),
			Refresh: uint32(_timeout),
			Retry:   uint32(_timeout / 4),
			Expire:  uint32(_timeout * 2),
			Minttl:  uint32(_timeout / 2),
		},
	}
}

func ns() []dns.RR {
	return []dns.RR{
		&dns.NS{
			Hdr: dns.RR_Header{
				Name:   _domainDot,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    uint32(_timeout),
			},
			Ns: "ns1." + _domainDot,
		}, &dns.NS{
			Hdr: dns.RR_Header{
				Name:   _domainDot,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    uint32(_timeout),
			},
			Ns: "ns2." + _domainDot,
		},
	}
}

func makeDNSHandler() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		start := time.Now()
		fmt.Println("got dns message for:", r.Question[0].Name)
		fmt.Println("message from: ", w.RemoteAddr())

		msg := new(dns.Msg)

		msg.SetReply(r)
		msg.Authoritative = true
		msg.Ns = ns()

		q := r.Question[0]

		switch q.Qtype {
		case dns.TypeNS:
			msg.Answer = ns()
		case dns.TypeSOA:
			msg.Answer = soa()
		case dns.TypeCNAME:
			fallthrough
		case dns.TypeA:
			fallthrough
		case dns.TypeAAAA:
			rr := query(q.Name)
			if len(rr) != 0 {
				msg.Answer = rr
			} else {
				msg.Answer = soa()
			}
		}

		w.WriteMsg(msg)
		fmt.Println("end: ", time.Now().Sub(start))
		fmt.Println("responding: ", msg.String())
	}
}
