package main

import (
	"fmt"
	"github.com/miekg/dns"
	"time"
)

func serial() uint32 {
	t := time.Now()

	yyyymmdd := (t.Year() * 10000) + (int(t.Month()) * 100) + (t.Day())
	return uint32(yyyymmdd * 100) + uint32(_changes)
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

// ear7h.net.	120	IN	SOA	ns1.ear7h.net. julio\.grillo98.gmail.com 2017271001 120 30 360 60
// google.com.		59	IN	SOA	ns1.google.com. dns-admin.google.com. 175207927 900 900 1800 60

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
			Mbox:    "julio\\.grillo98.gmail.com.",
			Serial:  serial(),
			Refresh: uint32(_timeout),
			Retry:   uint32(_timeout / 4),
			Expire:  uint32(_timeout * 6),
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
		msg := new(dns.Msg)

		msg.SetReply(r)
		msg.Authoritative = true
		msg.Ns = ns()

		q := r.Question[0]
		fmt.Println(q.String())

		switch q.Qtype {
		case dns.TypeNS:
			msg.Answer = ns()
		case dns.TypeSOA:
			msg.Answer = soa()
		case dns.TypeA, dns.TypeCNAME, dns.TypeAAAA:
			rr := query(q.Name)
			if len(rr) > 0 {
				msg.Answer = rr
			} else {
				msg.Answer = soa()
			}
		default:
			msg.Answer = soa()
		}

		err := w.WriteMsg(msg)
		if err != nil {
			fmt.Println(err)
		}
	}
}
